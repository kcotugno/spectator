package main

import (
	"errors"
)

type Queue struct {
	data         []interface{}
	begin, end   int
	length, size int
}

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

func (q *Queue) Enqueue(v interface{}) error {
	if q.length == 256 {
		return errors.New("Queue Full")
	}

	q.end++

	if len(q.data) == q.end {
		q.end = 0
	}

	q.length++
	q.data[q.end] = v

	return nil
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

	if i > q.length {
		return nil
	}

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
