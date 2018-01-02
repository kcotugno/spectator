package main

import (
	"git.kevincotugno.com/kcotugno/spectator/exhibit"
	"github.com/shopspring/decimal"

	"image"
	"log"
	"time"
	"sync"
	"unicode/utf8"
)

const (
	coin       = "ETH-USD"
	timeFormat = "15:04:05"
)

var trades = NewQueue()

var terminal *exhibit.Terminal
var ob *OrderBook

var window *exhibit.WindowWidget
var topAsks *exhibit.ListWidget
var topBids *exhibit.ListWidget
var midPrice *exhibit.ListWidget
var history *exhibit.ListWidget

var numLock sync.Mutex
var num     int

var sizeLock    sync.Mutex
var sizeChanged bool

var low, high decimal.Decimal

func main() {
	var err error

	terminal = exhibit.Init()
	defer terminal.Shutdown()
	terminal.HideCursor()

	window = &exhibit.WindowWidget{}
	window.SetBorder(exhibit.Border{Visible: true, Attributes: exhibit.Attributes{ForegroundColor: exhibit.FGYellow}})

	topAsks = &exhibit.ListWidget{}
	topBids = &exhibit.ListWidget{}

	midPrice = &exhibit.ListWidget{}
	midPrice.SetSize(image.Pt(24, 1))

	history = &exhibit.ListWidget{}

	window.AddWidget(topAsks)
	window.AddWidget(midPrice)
	window.AddWidget(topBids)
	window.AddWidget(history)

	scene := exhibit.Scene{terminal, window}

	watchSize(terminal)

	ob, err = NewOrderBook(coin)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		Loop:
		for e := range terminal.Event {
			switch e {
			case exhibit.Eventq:
				fallthrough
			case exhibit.EventCtrC:
				ob.Shutdown()
				break Loop
			}
		}
	}()

	go renderLoop(&scene, 100*time.Millisecond)

	updateOrders("sell")
	updateOrders("buy")

	for msg := range ob.Msg {
		updateOrders(msg.Side)

		if msg.Type == "match" {
			addTrade(msg)
		}
	}
}

func numPerSide() int {
	numLock.Lock()
	defer numLock.Unlock()

	return num
}

func numOfOrderPerSide(y int) int {
	total := y - 3 - 2

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

func renderLoop(scene *exhibit.Scene, interval time.Duration) {
	timer := time.NewTicker(interval)
	changed := time.NewTicker(2 * time.Second)

	for {
		select {
		case <-changed.C:
			if didsizeChanged() {
				terminal.Clear()
				setSizeChanged(false)
			}
		case <-timer.C:
			scene.Render()
		}
	}
}

func recalcSizes(sz image.Point) {
	numLock.Lock()
	defer numLock.Unlock()

	sizeLock.Lock()
	defer sizeLock.Unlock()

	num = numOfOrderPerSide(sz.Y)

	if history.Size() != image.Pt(33, sz.Y) {
		history.SetSize(image.Pt(33, sz.Y))
	}

	hOr := image.Pt(sz.X-35, 0)
	if history.Origin() != hOr {
		history.SetOrigin(hOr)
	}

	num := numOfOrderPerSide(sz.Y)
	size := image.Point{23, num}
	if topAsks.Size() != size {
		topAsks.SetSize(size)
	}

	bOrigin := image.Pt(0, size.Y+3)
	if topBids.Origin() != bOrigin {
		topBids.SetOrigin(bOrigin)
	}
	if topBids.Size() != size {
		topBids.SetSize(size)
	}

	mOr := image.Pt(0, num+1)
	if midPrice.Origin() != mOr {
		midPrice.SetOrigin(mOr)
	}
}

func padString(value string, length int) string {
	c := utf8.RuneCountInString(value)

	if c >= length {
		return value
	}

	pad := length - c

	var s string
	for i := 0; i < pad; i++ {
		s = s + " "
	}

	return s + value
}

func fmtObEntry(price, size decimal.Decimal) string {
	s := padString(price.StringFixed(2), 8)
	s = s + " "
	s = s + padString(size.StringFixed(8), 14)

	return s
}

func fmtHistoryEntry(msg Message) string {
	var arrow string
	switch msg.Side {
	case "buy":
		arrow = "↓"
	case "sell":
		arrow = "↑"
	}

	s := padString(msg.Size.StringFixed(8), 14)
	s = s + " "
	s = s + padString(msg.Price.StringFixed(2), 8)
	s = s + arrow
	s = s + " "
	s = s + msg.Time.Local().Format(timeFormat)

	return s
}

func fmtMid(high, low decimal.Decimal) string {
	diff := low.Sub(high)
	mid := high.Add(diff.Div(decimal.New(2, 0))).StringFixed(3)

	return padString(mid, 9) + padString(diff.StringFixed(2), 14)
}

func watchSize(t *exhibit.Terminal) {
	go func() {
		for s := range t.SizeChange {
			recalcSizes(s)
			setSizeChanged(true)
		}
	}()
}

func updateOrders(side string) {
	n := numPerSide()
	entries := ob.Entries(side, n)

	switch side {
	case "sell":
		updateAsks(entries)
	case "buy":
		updateBids(entries)
	}

	midPrice.AddEntry(ListEntry{Value: fmtMid(high, low)})
	midPrice.Commit()
}

func updateAsks(entries []Entries) {
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		price, size := flatten(entry)

		topAsks.AddEntry(ListEntry{Value: fmtObEntry(price, size),
			Attrs: exhibit.Attributes{ForegroundColor:
			exhibit.FGRed}})

		if i == 0 {
			low = price
		}
	}

	topAsks.Commit()
}

func updateBids(entries []Entries) {
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		price, size := flatten(entry)

		topBids.AddEntry(ListEntry{Value: fmtObEntry(price, size),
			Attrs: exhibit.Attributes{ForegroundColor: exhibit.FGGreen}})

		if i == 0 {
			high = price
		}
	}

	topBids.Commit()
}

func addTrade(msg Message) {
	if trades.Length() == 256 {
		trades.Dequeue()
	}

	trades.Enqueue(msg)

	max := history.Size().Y
	length := trades.Length()
	var num int
	if length > max {
		num = max
	} else {
		num = length
	}

	for i := 0; i < num; i++ {
		var index int

		adj := trades.Length() - i - 1

		if adj < 0 {
			break
		} else {
			index = adj
		}

		e := trades.Element(index)

		if e != nil {
			msg := e.(Message)

			var attrs exhibit.Attributes

			switch msg.Side {
			case "buy":
				attrs.ForegroundColor = exhibit.FGRed
			case "sell":
				attrs.ForegroundColor = exhibit.FGGreen
			}

			le := ListEntry{fmtHistoryEntry(msg), attrs}
			history.AddEntry(le)
		}
	}

	history.Commit()
}

func didsizeChanged() bool {
	sizeLock.Lock()
	defer sizeLock.Unlock()

	return sizeChanged
}

func setSizeChanged(c bool) {
	sizeLock.Lock()
	defer sizeLock.Unlock()

	sizeChanged = c
}
