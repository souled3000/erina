package main

import (
	"fmt"
	"octopus/common"
)

func main() {
	f1()
}

func f1() {
	q := common.NewQueue()
	fmt.Println(q.Size())
	q.Put("1")
	q.Put("2")
	q.Put("3")
	fmt.Println(q.Size())
	a := q.Next()
	fmt.Println(a, q.Size())
	a = q.Next()
	fmt.Println(a, q.Size())
	a = q.Next()
	fmt.Println(a, q.Size())
	a = q.Next()
	fmt.Println(a, q.Size())
	a = q.Next()
	fmt.Println(a, q.Size())
	a = q.Next()
	fmt.Println(a, q.Size())
	fmt.Println(q.Contains("3"))
	b := make([]interface{}, 3)
	b[0] = "1"
	b[1] = "2"
	b[2] = "3"
	q.Retrain(b)
	fmt.Println(q.Size())
	q.Remove(b[0])
	fmt.Println(q.Size())
	q.Remove(b[1])
	fmt.Println(q.Size())
	q.Remove(b[2])
	fmt.Println(q.Size())
	q.Remove(b[2])
	fmt.Println(q.Size())
}
