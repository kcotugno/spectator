package main

import (
	"github.com/emirpasic/gods/trees/redblacktree"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"golang.org/x/sys/unix"

	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
	"unicode/utf8"
)

type Queue struct {
	data         []interface{}
	begin, end   int
	length, size int
}

type Terminal struct {
	Event <-chan Event

	in *os.File

	outLock sync.Mutex
	out     *os.File

	bufLock sync.Mutex
	buffer  *bytes.Buffer

	sizeLock sync.Mutex
	size     Size

	displayLock sync.Mutex
	display     [][]Cell

	interLock sync.Mutex
	interBuf  []Cell

	currentAttributes Attributes
	cursorPosition    Position
	cursorVisible     bool

	termios unix.Termios

	doneLock sync.Mutex
	done     bool

	shutdown chan struct{}
	event    chan Event
}

type Cell struct {
	Pos   Position
	Value rune
	Attrs Attributes
}

type Attributes struct {
	ForegroundColor ForegroundColor
	BackgroundColor BackgroundColor
	Bold            bool
	Italics         bool
	Blink           bool
	Underline       bool
}

type Scene struct {
	Terminal *Terminal
	Window   Widget
}

type Widget interface {
	Render() [][]Cell
}

type WindowWidget struct {
	Constraints Constraints
	Border      Border

	widgets []Widget
}

type ListWidget struct {
	Constraints Constraints
	Attrs       Attributes

	cellLock sync.Mutex
	cells    [][]Cell

	rightAlign bool
	border     bool

	listLock sync.Mutex
	list     [][]rune

	lastSize Size
}

type Constraints struct {
	Top    bool
	Bottom bool
	Left   bool
	Right  bool
}

type Border Constraints

type ForegroundColor int
type BackgroundColor int
type Event byte

type Sub struct {
	Type       string   `json:"type"`
	ProductIds []string `json:"product_ids"`
	Channels   []string `json:"channels"`
}

type Message struct {
	Sequence      int64           `json:"sequence"`
	Type          string          `json:"type"`
	Side          string          `json:"side"`
	Price         decimal.Decimal `json:"price"`
	Size          decimal.Decimal `json:"size"`
	OrderId       string          `json:"order_id"`
	MakerOrderId  string          `json:"maker_order_id"`
	RemainingSize decimal.Decimal `json:"remaining_size"`
	NewSize       decimal.Decimal `json:"new_size"`
	ProductId     string          `json:"product_id"`
	Time          time.Time       `json:"time"`
	Reason        string          `json:"reason"`
	OrderType     string          `json:"order_type"`
	ClientOid     string          `json:"client_oid"`
}

type LevelThree struct {
	Sequence int64             `json:"sequence"`
	Bids     []LevelThreeEntry `json:"bids"`
	Asks     []LevelThreeEntry `json:"asks"`
}

type Entry struct {
	Id    string
	Price decimal.Decimal
	Size  decimal.Decimal
}

type Position struct {
	X int
	Y int
}

type Size Position

type Entries map[string]Entry

type LevelThreeEntry []string

const (
	CtrC = Event(3)
)

const (
	Normal = iota
	Bold
	_
	_
	Underline
	SlowBlink
)

const (
	FGBlack = ForegroundColor(iota + 30)
	FGRed
	FGGreen
	FGYellow
	FGBlue
	FGMagenta
	FGCyan
	FGWhite
)

const (
	BGBlack = BackgroundColor(iota + 40)
	BGRed
	BGGreen
	BGYellow
	BGBlue
	BGMagenta
	BGCyan
	BGWhite
)

const (
	coin = "ETH-USD"

	timeFormat = "15:04:05"
	smcup      = "\x1b[?1049h"
	rmcup      = "\x1b[?1049l"
	civis      = "\x1b[?25l"
	cvvis      = "\x1b[?12;25h"
	clear      = "\x1b[2J"
	sgr        = "\x1b[%vm"
	cup        = "\x1b[%v;%vH"
)

var sub = Sub{"subscribe", []string{coin}, []string{"full"}}

var asks = redblacktree.NewWith(DecimalComparator)
var bids = redblacktree.NewWith(ReverseDecimalComparator)
var trades = NewQueue()

var terminal *Terminal

var window *WindowWidget
var topAsks *ListWidget
var topBids *ListWidget
var midPrice *ListWidget

func NewQueue() *Queue {
	q := Queue{}
	q.data = make([]interface{}, 256)
	q.size = 256
	q.end = -1

	return &q
}

func (q *Queue) Length() int {
	return q.length
}

func (q *Queue) Enqueue(v interface{}) {
	if q.length == 256 {
		fmt.Println("Queue Full")
		os.Exit(1)
	}

	q.end++

	if len(q.data) == q.end {
		q.end = 0
	}

	q.length++
	q.data[q.end] = v
}

func (q *Queue) Dequeue() interface{} {
	if q.length == 0 {
		return nil
	}

	v := q.data[q.begin]

	q.begin++
	q.length--

	if q.size == q.begin {
		q.begin = 0
	}

	if q.length == 0 {
		q.begin = 0
		q.end = -1
	}

	return v
}

func (q *Queue) Element(i int) interface{} {
	var v interface{}

	if q.begin == 0 {
		v = q.data[i]
	} else {
		if q.begin+i >= q.size {
			j := (q.begin + i) - q.size
			v = q.data[j]
		} else {
			v = q.data[q.begin+i]
		}
	}

	return v
}

func main() {
	terminal = Init()
	defer terminal.Shutdown()
	terminal.HideCursor()

	window = &WindowWidget{}
	topAsks = &ListWidget{}
	window.Constraints.Bottom = true
	topAsks.SetBorder(true)
	topAsks.SetRightAlign(true)
	topAsks.Attrs.ForegroundColor = FGRed

	topBids = &ListWidget{}
	topBids.SetBorder(true)
	topBids.SetRightAlign(true)
	topBids.Attrs.ForegroundColor = FGGreen

	midPrice = &ListWidget{}
	midPrice.SetRightAlign(true)

	window.AddWidget(topAsks)
	//         window.AddWidget(midPrice)
	//         window.AddWidget(topBids)

	scene := Scene{terminal, window}

	conn, _, err := websocket.DefaultDialer.Dial("wss://ws-feed.gdax.com", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	go func() {
		for e := range terminal.Event {
			if e == CtrC {
				conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.
						CloseNormalClosure, ""))
				break
			}
		}
	}()

	err = conn.WriteJSON(sub)
	if err != nil {
		log.Fatal(err)
	}

	go renderLoop(&scene, 100*time.Millisecond)

	sequence := loadOrderBook()

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				break
			} else {
				log.Fatal(err)
			}
		}

		if msg.Type == "subscriptions" {
			continue
		}

		if msg.Sequence <= sequence {
			continue
		}

		if msg.Sequence != sequence+1 {
			sequence = loadOrderBook()
			continue
		}

		sequence = msg.Sequence

		switch msg.Type {
		case "received":
		case "open":
			open(msg)
		case "done":
			done(msg)
		case "match":
			match(msg)
		case "change":
			change(msg)
		default:
			log.Fatal("Unknown message type")
		}

		sz := terminal.Size()

		num := numOfOrderPerSide(sz.Y)

		aIt := asks.Iterator()
		s := make([]string, num)

		var low, high decimal.Decimal
		for i := num - 1; i >= 0; i-- {
			aIt.Next()

			entries := aIt.Value().(Entries)
			price, size := flatten(entries)

			s[i] = fmt.Sprintf("%v - %v ", price.StringFixed(2), size.StringFixed(8))

			if i == num-1 {
				low = price
			}
		}
		topAsks.SetList(append([]string{}, s...))

		bIt := bids.Iterator()
		for i := 0; i < num; i++ {
			bIt.Next()

			entries := bIt.Value().(Entries)
			price, size := flatten(entries)

			s[i] = fmt.Sprintf("%v - %v ", price.StringFixed(2), size.StringFixed(8))

			if i == 0 {
				high = price
			}
		}

		topBids.SetList(append([]string(nil), s...))

		diff := low.Sub(high)

		s = []string{"", fmt.Sprintf("%v  Spread: %v",
			high.Add(diff.Div(decimal.New(2, 0))).StringFixed(3),
			diff.StringFixed(2)), ""}

		midPrice.SetList(append([]string(nil), s...))
	}
}

func open(msg Message) {
	tree := sideTree(msg.Side)

	var entries Entries
	var entry Entry

	entries, ok := treeEntries(tree, msg.Price)
	if !ok {
		entries = Entries{}

		tree.Put(msg.Price, entries)
	}

	entry.Id = msg.OrderId
	entry.Price = msg.Price
	entry.Size = msg.RemainingSize

	entries[entry.Id] = entry
}

func done(msg Message) {
	if msg.OrderType == "market" {
		return
	}
	tree := sideTree(msg.Side)

	entries, ok := treeEntries(tree, msg.Price)
	if !ok {
		return
	}

	delete(entries, msg.OrderId)
	if len(entries) == 0 {
		tree.Remove(msg.Price)
	} else {
		tree.Put(msg.Price, entries)
	}
}

func match(msg Message) {
	tree := sideTree(msg.Side)

	entries, ok := treeEntries(tree, msg.Price)
	if !ok {
		return
	}

	entry, ok := entries[msg.MakerOrderId]
	if !ok {
		return
	}

	entry.Size = entry.Size.Sub(msg.Size)
	entries[msg.MakerOrderId] = entry

	if trades.Length() == 256 {
		trades.Dequeue()
	}

	trades.Enqueue(msg)
}

func change(msg Message) {
	tree := sideTree(msg.Side)

	entries, ok := treeEntries(tree, msg.Price)
	if !ok {
		return
	}

	entry, ok := entries[msg.OrderId]
	if !ok {
		return
	}

	entry.Size = msg.NewSize
	entries[msg.OrderId] = entry
}

func loadOrderBook() int64 {
	bids.Clear()
	asks.Clear()

	resp, err := http.Get(fmt.Sprintf("https://api.gdax.com/products/%v/book?level=3", coin))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var parsed LevelThree

	err = json.Unmarshal(buf, &parsed)
	if err != nil {
		log.Fatal(err)
	}

	for _, e := range parsed.Bids {
		var entry Entry

		entry.Price, err = decimal.NewFromString(e[0])
		if err != nil {
			log.Fatal(err)
		}
		entry.Size, err = decimal.NewFromString(e[1])
		if err != nil {
			log.Fatal(err)
		}
		entry.Id = e[2]

		var entries Entries
		values, ok := bids.Get(entry.Price)
		if !ok {
			entries = Entries{}

			bids.Put(entry.Price, entries)
		} else {
			entries = values.(Entries)
		}

		entries[entry.Id] = entry
	}

	for _, e := range parsed.Asks {
		var entry Entry

		entry.Price, err = decimal.NewFromString(e[0])
		if err != nil {
			log.Fatal(err)
		}
		entry.Size, err = decimal.NewFromString(e[1])
		if err != nil {
			log.Fatal(err)
		}
		entry.Id = e[2]

		var entries Entries
		values, ok := asks.Get(entry.Price)
		if !ok {
			entries = Entries{}

			asks.Put(entry.Price, entries)
		} else {
			entries = values.(Entries)
		}

		entries[entry.Id] = entry
	}

	return parsed.Sequence
}

func sideTree(side string) *redblacktree.Tree {
	switch side {
	case "buy":
		return bids
	case "sell":
		return asks
	}

	return nil
}

func treeEntries(tree *redblacktree.Tree, key decimal.Decimal) (Entries, bool) {
	values, ok := tree.Get(key)

	if ok {
		return values.(Entries), true
	} else {
		return nil, false
	}
}

func numOfOrderPerSide(y int) int {
	total := y - 3 - 4

	return (total / 2)
}

func flatten(entries Entries) (decimal.Decimal, decimal.Decimal) {
	var price, size decimal.Decimal

	for _, v := range entries {
		price = v.Price
		size = size.Add(v.Size)
	}

	return price, size
}

func renderLoop(scene *Scene, interval time.Duration) {
	timer := time.NewTicker(interval)

	for {
		select {
		case <-timer.C:
			scene.Render()
		}
	}
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

	termios, err := unix.IoctlGetTermios(int(out.Fd()), unix.TCGETS)
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

	x, y := t.sizeInternal()
	if x != 0 && y != 0 {
		t.setSize(x, y)
	}

	t.enterAlt()
	t.enterRaw()
	t.SetCursor(0, 0)
	t.Render()

	t.watchSize()
	t.watchInput()

	return &t
}

func (t *Terminal) Shutdown() {
	t.shutdown <- struct{}{}

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
	close(t.event)
}

func (t *Terminal) Clear() {
	t.writeBuffer([]byte(clear))
}

func (t *Terminal) SetCursor(x, y int) {
	t.writeBuffer([]byte(fmt.Sprintf(cup, y+1, x+1)))
	t.cursorPosition.X = x
	t.cursorPosition.Y = y
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

func (t *Terminal) Size() Size {
	t.sizeLock.Lock()
	defer t.sizeLock.Unlock()

	return t.size
}

func (t *Terminal) setSize(x, y int) {
	t.sizeLock.Lock()
	defer t.sizeLock.Unlock()

	if t.size.X == x && t.size.Y == y {
		return
	}

	t.displayLock.Lock()
	defer t.displayLock.Unlock()

	t.size.X = x
	t.size.Y = y

	if t.display == nil {
		t.display = make([][]Cell, x)
	} else if len(t.display) < x {
		t.display = append(t.display, make([][]Cell, x-len(t.display))...)
	}

	for i := 0; i < x; i++ {
		if t.display[i] == nil {
			t.display[i] = make([]Cell, y)
		} else if len(t.display[i]) < y {
			t.display[i] = append(t.display[i],
				make([]Cell, y-len(t.display[i]))...)
		}
	}

	t.Clear()
}

func (t *Terminal) WriteString(s string, x, y int, attrs Attributes) {
	if len(s) == 0 || len(t.display) < x+1 || len(t.display[0]) < y+1 {
		return
	}

	var j int
	for i := 0; i < len(s); i++ {
		r, sz := utf8.DecodeRuneInString(s[j:])

		if sz > 0 {

			cell := Cell{Position{x + i, y}, r, attrs}

			if t.display[x+i][y] != cell {
				t.interBuf = append(t.interBuf, cell)
			}

			j = j + sz
		}
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
	sz := t.Size()

	for _, c := range t.interBuf {
		if c.Pos.X >= sz.X || c.Pos.Y >= sz.Y {
			continue
		}

		t.display[c.Pos.X][c.Pos.Y] = c

		if t.currentAttributes != c.Attrs {
			changed = true
			t.writeAttrs(c.Attrs)
		}

		if t.cursorPosition.X+1 != c.Pos.X {
			t.SetCursor(c.Pos.X, c.Pos.Y)
		}

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

	if t.cursorPosition.X+1 <= t.size.X {
		t.cursorPosition.X = 1
	} else {
		t.cursorPosition.X++
	}

	if t.cursorPosition.Y+1 <= t.size.Y {
		t.cursorPosition.Y = 1
	} else {
		t.cursorPosition.Y++
	}
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
	}()
}

func (t *Terminal) watchSize() {
	go func() {
		timer := time.NewTicker(1 * time.Second)

		for {
			select {
			case <-t.shutdown:
				timer.Stop()
				break
			case <-timer.C:
				x, y := t.sizeInternal()
				if x != 0 && y != 0 {
					t.setSize(x, y)
				}

				continue
			}

			break
		}
	}()
}

func (t *Terminal) sizeInternal() (x, y int) {
	sz, err := unix.IoctlGetWinsize(int(t.in.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0
	}

	return int(sz.Col), int(sz.Row)
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

	err := unix.IoctlSetTermios(t.outFd(), unix.TCSETS, &termios)
	if err != nil {
		t.exitAlt()
		t.out.Close()
		t.in.Close()
		log.Panic(err)
	}
}

func (t *Terminal) exitRaw() {
	err := unix.IoctlSetTermios(t.outFd(), unix.TCSETS, &t.termios)
	if err != nil {
		t.out.Close()
		t.in.Close()
		log.Panic(err)
	}
}

func (s *Scene) Render() {
	c := make([]Cell, 0)

	for _, row := range s.Window.Render() {
		for _, col := range row {
			c = append(c, col)
		}
	}

	s.Terminal.WriteCells(c)
	s.Terminal.Render()
}

func (w *WindowWidget) AddWidget(widget Widget) {
	if w.widgets == nil {
		w.widgets = make([]Widget, 0)
	}

	w.widgets = append(w.widgets, widget)
}

func (w *WindowWidget) Render() [][]Cell {
	c := make([][]Cell, 0)

	var y int
	for _, w := range w.widgets {
		for _, row := range w.Render() {
			t := make([]Cell, len(row))
			c = append(c, t)

			for j, col := range row {
				col.Pos.Y = y
				c[y][j] = col
			}

			y++
		}
	}

	return c
}

func (l *ListWidget) Render() [][]Cell {
	l.cellLock.Lock()
	defer l.cellLock.Unlock()

	var sx int
	sy := len(l.cells)

	if sy > 0 {
		sx = len(l.cells[0])
	} else {
		return make([][]Cell, 0)
	}

	dx := 0
	dy := 0
	if l.lastSize.X > sx {
		dx = l.lastSize.X - sx
	}

	if l.lastSize.Y > sy {
		dy = l.lastSize.Y - sy
	}

	for y := 0; y < sy+dy; y++ {
		if y >= sy {
			l.cells = append(l.cells, []Cell{})
		}

		for x := 0; x < sx+dx; x++ {
			if x >= sx || y >= sy {
				c := Cell{}
				c.Pos.X = x
				c.Pos.Y = y
				c.Value = ' '
				l.cells[y] = append(l.cells[y], c)
			} else {
				l.cells[y][x].Attrs = l.Attrs
				l.cells[y][x].Pos.X = x
				l.cells[y][x].Pos.Y = y
			}
		}
	}

	l.lastSize.X = sx
	l.lastSize.Y = sy
	return append([][]Cell(nil), l.cells...)
}

func (l *ListWidget) SetList(list []string) {
	buf := make([][]rune, len(list))

	for i, s := range list {
		buf[i] = make([]rune, 0)

		for _, r := range s {
			buf[i] = append(buf[i], r)
		}
	}

	l.listLock.Lock()
	l.list = buf
	l.listLock.Unlock()

	l.recalculateCells()
}

func (l *ListWidget) SetBorder(b bool) {
	if l.border == b {
		return
	}

	l.border = b

	l.recalculateCells()
}

func (l *ListWidget) SetRightAlign(b bool) {
	if l.rightAlign == b {
		return
	}

	l.rightAlign = b

	l.recalculateCells()
}

func (l *ListWidget) recalculateCells() {
	l.listLock.Lock()
	defer l.listLock.Unlock()

	var longest int
	var border int

	for _, s := range l.list {
		if longest < len(s) {
			longest = len(s)
		}
	}

	cells := make([][]Cell, len(l.list))

	if l.border {
		border = 1

		top := make([]Cell, longest+2)
		top[0] = Cell{Value: '┏'}
		top[longest+1] = Cell{Value: '┓'}
		for i := 1; i <= longest; i++ {
			top[i] = Cell{Value: '━'}
		}

		bottom := append([]Cell(nil), top...)
		bottom[0] = Cell{Value: '┗'}
		bottom[longest+1] = Cell{Value: '┛'}

		cells = append([][]Cell{top}, cells...)
		cells = append(cells, bottom)
	}

	for i, s := range l.list {
		cells[i+border] = make([]Cell, longest+border+border)

		var start int
		if l.rightAlign {
			start = longest - len(s)
		} else {
			start = 0
		}

		if l.border {
			c := Cell{Value: '┃'}
			cells[i+border][0] = c
			cells[i+border][longest+1] = c
		}

		for j := 0; j < longest; j++ {
			c := Cell{}
			if j > start+len(s)-1 || j < start {
				c.Value = ' '
			} else {
				c.Value = s[j-start]
			}

			cells[i+border][j+border] = c
		}
	}

	l.cellLock.Lock()
	defer l.cellLock.Unlock()

	l.cells = cells
}

func DecimalComparator(a, b interface{}) int {
	aAsserted := a.(decimal.Decimal)
	bAsserted := b.(decimal.Decimal)

	switch {
	case aAsserted.GreaterThan(bAsserted):
		return 1
	case aAsserted.LessThan(bAsserted):
		return -1
	default:
		return 0
	}
}

func ReverseDecimalComparator(a, b interface{}) int {
	aAsserted := a.(decimal.Decimal)
	bAsserted := b.(decimal.Decimal)

	switch {
	case aAsserted.GreaterThan(bAsserted):
		return -1
	case aAsserted.LessThan(bAsserted):
		return 1
	default:
		return 0
	}
}
