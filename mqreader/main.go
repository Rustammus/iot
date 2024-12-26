package main

import (
	"bufio"
	"context"
	mq "github.com/eclipse/paho.mqtt.golang"
	"log"
	"os"
	"os/signal"
	"time"
)

const mainT = "ghisjlgoc/"

func main() {
	opt := mq.NewClientOptions().SetClientID("balabol2").AddBroker("tcp://test.mosquitto.org:1884").SetUsername("rw").SetPassword("readwrite")
	//opt := mq.NewClientOptions().SetClientID("balabol2").AddBroker("tcp://test.mosquitto.org:1883")
	client := mq.NewClient(opt)
	defer client.Disconnect(0)

	maxRetry := 10
	for !client.IsConnected() && maxRetry > 0 {
		maxRetry--
		tkt := client.Connect()
		tkt.Wait()
		log.Println(tkt.Error())
		log.Println("mqtt client is not connected")
		time.Sleep(1 * time.Second)
	}
	if !client.IsConnected() {
		log.Fatal("mqtt can not connect")
	} else {
		log.Println("mqtt client is connected")
	}

	ctx, cl := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cl()
	client.Subscribe(mainT+"air_quality", 2, func(client mq.Client, msg mq.Message) {
		log.Print("Receive msg: ", string(msg.Payload()))
	})

	log.Println("waiting for signal")
	go sender(client)
	log.Print(<-ctx.Done())
}

func sender(c mq.Client) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := scanner.Text()

		tk := c.Publish(mainT+"update", 2, false, msg)
		tk.Wait()
		err := tk.Error()
		if err != nil {
			log.Print("error on send msg: ", err)
		}
		log.Println("send msg: ", msg)
	}
}
