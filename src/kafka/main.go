package main

import (
	"encoding/hex"
	"fmt"
	"octopus/srv"
	"time"
)

func main() {
	f1()
	f2()
}
func f2() {
	ser := srv.KafkaProducer([]string{"193.168.1.115:9092"})
	time.Sleep(3 * time.Second)
	m := srv.KafkaMsg{}
	m.Topic = "test"
	m.Msg, _ = hex.DecodeString("c500010012f9e13433cbed061ecbed06193412f9e13412f9e33403f9ea3446f400000000000000006d6408000011010000001a")
	m.Msg = []byte("llllllllllllllllllll")
	ser.ProducerCh <- m
	m.Topic = "his_dev"
	ser.ProducerCh <- m
	time.Sleep(5 * time.Second)
}
func f1() {
	ser := srv.KafkaConsumer([]string{"193.168.1.115:9092"}, "test,his_dev")
	go func() {
		for {
			m := <-ser.ConsumerCh
			fmt.Println("recv", m)
		}
	}()
}
