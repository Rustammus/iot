package main

import (
	mq "github.com/eclipse/paho.mqtt.golang"
	"log"
	"strconv"
)

const mainT = "ghisjlgoc/"

type MQClient struct {
	c          mq.Client
	updateChan chan<- string
}

func NewMQ(autoConnect bool) *MQClient {
	opt := mq.NewClientOptions().SetClientID("balabol").AddBroker("tcp://test.mosquitto.org:1884").SetUsername("rw").SetPassword("readwrite")
	//opt := mq.NewClientOptions().SetClientID("balabol").AddBroker("tcp://test.mosquitto.org:1883")
	client := mq.NewClient(opt)

	if autoConnect {
		client.Connect()
	}

	return &MQClient{c: client}
}

func (mc *MQClient) Connect() {
	tkt := mc.c.Connect()
	tkt.Wait()
	err := tkt.Error()
	if err != nil {
		log.Print("Error on connect to mqtt server: ", err)
		return
	}
	log.Println("connected to mqtt server")

	mc.StartReading(mc.updateChan)
}

func (mc *MQClient) Disconnect() {
	mc.c.Unsubscribe(mainT + "update")
	mc.c.Disconnect(250)
	log.Println("disconnected from mqtt server")
}

func (mc *MQClient) SendAirQuality(ppm int) {
	if !mc.c.IsConnected() {
		return
	}
	tk := mc.c.Publish(mainT+"air_quality", 2, false, strconv.FormatInt(int64(ppm), 10))
	// tk := mc.c.Publish(mainT+"air_quality", 2, false, "gabenus")
	tk.Wait()
	err := tk.Error()
	if err != nil {
		log.Println("error on SendAirQuality", err)
	}
	log.Print("Send AirQuality ppm: ", ppm)
}

func (mc *MQClient) StartReading(ch chan<- string) {
	if !mc.c.IsConnected() {
		return
	}
	tk := mc.c.Subscribe(mainT+"update", 2, func(client mq.Client, msg mq.Message) {
		log.Print("get update: ", string(msg.Payload()))
		ch <- string(msg.Payload())
	})
	if err := tk.Error(); err != nil {
		log.Println("error on GetUpdate.Subscribe", err)
	}
}
