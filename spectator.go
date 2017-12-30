package main

import (
	"git.kevincotugno.com/kcotugno/spectator/exhibit"
	"github.com/emirpasic/gods/trees/redblacktree"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"

	"encoding/json"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const (
	coin       = "ETH-USD"
	timeFormat = "15:04:05"
)

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

type Entries map[string]Entry

type LevelThreeEntry []string

var sub = Sub{"subscribe", []string{coin}, []string{"full"}}

var asks = redblacktree.NewWith(DecimalComparator)
var bids = redblacktree.NewWith(ReverseDecimalComparator)
var trades = NewQueue()

var terminal *exhibit.Terminal

var window *exhibit.WindowWidget
var topAsks *exhibit.ListWidget
var topBids *exhibit.ListWidget
var midPrice *exhibit.ListWidget

func main() {
	terminal = exhibit.Init()
	defer terminal.Shutdown()
	terminal.HideCursor()

	window = &exhibit.WindowWidget{}
	window.SetBorder(exhibit.Border{Visible: true})
	topAsks = &exhibit.ListWidget{}
	//         topAsks.SetSize(image.Point{100, 100})
	topAsks.SetRightAlign(true)
	topAsks.SetAttributes(exhibit.Attributes{ForegroundColor: exhibit.FGCyan})

	topBids = &exhibit.ListWidget{}
	//         topBids.SetRightAlign(true)
	//         topBids.SetAttributes(exhibit.Attributes{ForegroundColor: exhibit.FGGreen})

	midPrice = &exhibit.ListWidget{}
	//         midPrice.SetRightAlign(true)

	window.AddWidget(topAsks)
	//         window.AddWidget(midPrice)
	//         window.AddWidget(topBids)

	scene := exhibit.Scene{terminal, window}

	conn, _, err := websocket.DefaultDialer.Dial("wss://ws-feed.gdax.com", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	go func() {
	Loop:
		for e := range terminal.Event {
			switch e {
			case exhibit.Eventq:
				fallthrough
			case exhibit.EventCtrC:
				conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.
						CloseNormalClosure, ""))
				break Loop
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
		aP := image.Point{14, num}
		if topAsks.Size() != aP {
			topAsks.SetSize(aP)
		}

		aIt := asks.Iterator()

		var low, high decimal.Decimal
		asks := make([]ListEntry, num)
		for i := 0; i < num; i++ {
			aIt.Next()

			entries := aIt.Value().(Entries)
			price, size := flatten(entries)

			asks[i] = ListEntry{Value: size.StringFixed(8),
				Attrs: exhibit.Attributes{ForegroundColor: exhibit.FGMagenta}}

			if i == 0 {
				low = price
			}
		}

		for i := num - 1; i >= 0; i-- {
			topAsks.AddEntry(asks[i])
		}

		topAsks.Commit()

		bIt := bids.Iterator()
		for i := 0; i < num; i++ {
			bIt.Next()

			entries := bIt.Value().(Entries)
			price, size := flatten(entries)

			topBids.AddEntry(ListEntry{Value: size.StringFixed(8)})

			if i == 0 {
				high = price
			}
		}

		topBids.Commit()

		diff := low.Sub(high)

		midPrice.AddEntry(ListEntry{Value: high.Add(diff.Div(decimal.
			New(2, 0))).StringFixed(3)})
		midPrice.Commit()
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

func renderLoop(scene *exhibit.Scene, interval time.Duration) {
	timer := time.NewTicker(interval)

	for {
		select {
		case <-timer.C:
			scene.Render()
		}
	}
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
