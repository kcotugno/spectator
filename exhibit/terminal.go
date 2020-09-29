package exhibit

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	smcup = "\x1b[?1049h"
	rmcup = "\x1b[?1049l"
	civis = "\x1b[?25l"
	cvvis = "\x1b[?12;25h"
	clear = "\x1b[2J"
	sgr   = "\x1b[%vm"
	cup   = "\x1b[%v;%vH"
)

type Terminal struct {
	Event      <-chan Event
	SizeChange <-chan image.Point

	in *os.File

	outLock sync.Mutex
	out     *os.File

	bufLock sync.Mutex
	buffer  *bytes.Buffer

	displayLock sync.Mutex
	display     Block

	interLock sync.Mutex
	interBuf  []Cell

	currentAttributes Attributes
	cursorVisible     bool

	termios unix.Termios

	doneLock sync.Mutex
	done     bool

	shutdown   chan struct{}
	event      chan Event
	sizeChange chan image.Point
}

func Init() *Terminal {
	var t Terminal

	out, err := os.OpenFile("/dev/tty", os.O_WRONLY|unix.O_NOCTTY, os.ModeCharDevice)
	if err != nil {
		log.Panic(err)
	}

	in, err := os.OpenFile("/dev/tty", os.O_RDONLY|unix.O_NOCTTY, os.ModeCharDevice)
	if err != nil {
		log.Panic(err)
	}

	termios, err := unix.IoctlGetTermios(int(out.Fd()), ioctlReadTermios)
	if err != nil {
		out.Close()
		in.Close()
		log.Panic(err)
	}

	t.buffer = &bytes.Buffer{}
	t.out = out
	t.in = in
	t.termios = *termios
	t.shutdown = make(chan struct{})
	t.event = make(chan Event, 1024)
	t.Event = t.event
	t.sizeChange = make(chan image.Point, 32)
	t.SizeChange = t.sizeChange

	size := t.sizeInternal()
	t.setSize(size)

	t.enterAlt()
	t.enterRaw()
	t.SetCursor(0, 0)
	t.Clear()
	t.Render()

	t.watchSize()
	t.watchInput()

	return &t
}

func (t *Terminal) Shutdown() {
	t.doneLock.Lock()
	t.done = true
	t.doneLock.Unlock()

	t.resetBuffer()
	t.ShowCursor()
	t.Render()

	t.exitRaw()
	t.exitAlt()

	t.out.Close()
	t.in.Close()

	t.shutdown <- struct{}{}
}

func (t *Terminal) Clear() {
	t.displayLock.Lock()
	defer t.displayLock.Unlock()
	t.display.Cells = make(map[image.Point]Cell)

	t.writeBuffer([]byte(clear))
}

func (t *Terminal) SetCursor(x, y int) {
	t.writeBuffer([]byte(fmt.Sprintf(cup, y+1, x+1)))
}

func (t *Terminal) ShowCursor() {
	t.writeBuffer([]byte(cvvis))
	t.cursorVisible = true
}

func (t *Terminal) HideCursor() {
	t.writeBuffer([]byte(civis))
	t.cursorVisible = false
}

func (t *Terminal) CursorVisible() bool {
	return t.cursorVisible
}

func (t *Terminal) Size() image.Point {
	t.displayLock.Lock()
	defer t.displayLock.Unlock()

	return t.display.Size()
}

func (t *Terminal) setSize(s image.Point) {
	t.displayLock.Lock()
	defer t.displayLock.Unlock()

	if t.display.Size().X == s.X && t.display.Size().Y == s.Y {
		return
	}

	t.display = NewBlock(0, 0, s.X, s.Y)

	select {
	case t.sizeChange <- s:
	default:
	}
}

func (t *Terminal) WriteCells(cells []Cell) {
	if len(cells) == 0 {
		return
	}

	t.interLock.Lock()
	defer t.interLock.Unlock()

	for _, cell := range cells {
		t.interBuf = append(t.interBuf, cell)
	}
}

func (t *Terminal) Render() {
	t.doneLock.Lock()
	defer t.doneLock.Unlock()

	if t.done {
		return
	}

	t.reconcileCells()
	t.flush()
	t.resetBuffer()
}

func (t *Terminal) writeBuffer(b []byte) {
	t.bufLock.Lock()
	defer t.bufLock.Unlock()

	t.buffer.Write(b)
}

func (t *Terminal) resetBuffer() {
	t.bufLock.Lock()
	defer t.bufLock.Unlock()

	t.buffer.Reset()
}

func (t *Terminal) flush() {
	t.bufLock.Lock()
	defer t.bufLock.Unlock()

	t.outLock.Lock()
	defer t.outLock.Unlock()

	io.Copy(t.out, t.buffer)
}

func (t *Terminal) reconcileCells() {
	t.displayLock.Lock()
	defer t.displayLock.Unlock()
	t.interLock.Lock()
	defer t.interLock.Unlock()

	var changed bool
	sz := t.display.Size()

	for _, c := range t.interBuf {
		if c.Point.X >= sz.X || c.Point.Y >= sz.Y {
			continue
		}

		if c.Point.X < 0 || c.Point.Y < 0 {
			continue
		}

		current := t.display.Cells[c.Point]

		if current.Value == c.Value && current.Attrs == c.Attrs {
			continue
		}

		t.display.Cells[c.Point] = c

		if t.currentAttributes != c.Attrs {
			changed = true
			t.writeAttrs(c.Attrs)
		}

		t.SetCursor(c.Point.X, c.Point.Y)
		t.writeRune(c.Value)
	}

	if changed {
		t.writeAttrs(Attributes{})
	}

	t.SetCursor(0, 0)

	t.interBuf = make([]Cell, 0)
}

func (t *Terminal) writeRune(r rune) {
	t.writeBuffer([]byte(string(r)))
}

func (t *Terminal) writeAttrs(attrs Attributes) {
	if t.currentAttributes.ForegroundColor != attrs.ForegroundColor {
		t.writeBuffer([]byte(fmt.Sprintf(sgr, attrs.ForegroundColor)))
		t.currentAttributes.ForegroundColor = attrs.ForegroundColor
	}

	if t.currentAttributes.BackgroundColor != attrs.BackgroundColor {
		t.writeBuffer([]byte(fmt.Sprintf(sgr, attrs.BackgroundColor)))
		t.currentAttributes.BackgroundColor = attrs.BackgroundColor
	}
}

func (t *Terminal) watchInput() {
	go func() {
		buf := make([]byte, 1)

		for {
			_, err := t.in.Read(buf)
			if err != nil {
				break
			}

			select {
			case t.event <- Event(buf[0]):
			default:
			}
		}

		close(t.event)
	}()
}

func (t *Terminal) watchSize() {
	go func() {
		timer := time.NewTicker(1 * time.Second)

		for {
			select {
			case <-t.shutdown:
				timer.Stop()
				close(t.sizeChange)
				break
			case <-timer.C:
				size := t.sizeInternal()
				t.setSize(size)

				continue
			}

			break
		}
	}()
}

func (t *Terminal) sizeInternal() image.Point {
	sz, err := unix.IoctlGetWinsize(int(t.in.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return image.Point{}
	}

	return image.Pt(int(sz.Col), int(sz.Row))
}

func (t *Terminal) writeOut(b []byte) {
	t.outLock.Lock()
	defer t.outLock.Unlock()

	t.out.Write(b)
}

func (t *Terminal) outFd() int {
	t.outLock.Lock()
	defer t.outLock.Unlock()

	return int(t.out.Fd())
}

func (t *Terminal) enterAlt() {
	t.writeOut([]byte(smcup))
	t.resetBuffer()
}

func (t *Terminal) exitAlt() {
	t.writeOut([]byte(rmcup))
	t.resetBuffer()
}

func (t *Terminal) enterRaw() {
	termios := t.termios

	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP |
		unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON

	termios.Oflag &^= unix.OPOST

	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG |
		unix.IEXTEN

	termios.Cflag &^= unix.CSIZE | unix.PARENB
	termios.Cflag |= unix.CS8
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0

	err := unix.IoctlSetTermios(t.outFd(), ioctlWriteTermios, &termios)
	if err != nil {
		t.exitAlt()
		t.out.Close()
		t.in.Close()
		log.Panic(err)
	}
}

func (t *Terminal) exitRaw() {
	err := unix.IoctlSetTermios(t.outFd(), ioctlWriteTermios, &t.termios)
	if err != nil {
		t.out.Close()
		t.in.Close()
		log.Panic(err)
	}
}
