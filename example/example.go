package main

import (
	"fmt"
	ipc "github.com/hoffigolang/golang-ipc"
	log "github.com/hoffigolang/golang-ipc/ipclogging"
	//m "github.com/hoffigolang/golang-ipc/msg"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"math"
	"time"
)

func main() {
	log.DoDebug = true

	//msg := m.NewStringMsg("hello World")
	//fmt.Println(msg.S())
	//syscall.Exit(0)

	serverConfig := &ipc.ServerConfig{
		SocketBasePath:    ipc.DefaultServerConfig.SocketBasePath,
		Timeout:           ipc.DefaultServerConfig.Timeout,
		MaxMsgSize:        ipc.DefaultServerConfig.MaxMsgSize,
		Encryption:        true,
		UnmaskPermissions: true, // true will make the socket writable for any user
	}
	clientConfig := &ipc.ClientConfig{
		SocketBasePath: ipc.DefaultClientConfig.SocketBasePath,
		Timeout:        ipc.DefaultClientConfig.Timeout,
		RetryTimer:     ipc.DefaultClientConfig.RetryTimer,
		MaxMsgSize:     ipc.DefaultClientConfig.MaxMsgSize,
		Encryption:     true,
	}

	waitForServerListening := make(chan bool)
	go server("example1", serverConfig, waitForServerListening)
	_ = <-waitForServerListening

	client("example1", clientConfig)
}

func client(ipcName string, clientConfig *ipc.ClientConfig) {
	start := time.Now()
	c, err := ipc.ClientDialAndHandshake("example1", clientConfig)
	if err != nil {
		log.Fatal(err)
		return
	}
	//c, err := ipc.DialAndHandshakeAsync("example1", clientConfig, nil)
	//if err != nil {
	//	log.Println(err)
	//	return
	//}
	//log.Println("client delaying before doing something")
	//for i := 1; i <= 2; i++ {
	//	time.Sleep(1 * time.Second)
	//	log.Printf("%dsecond", i)
	//}
	//err = c.StartProcessingMessages()
	//if err != nil {
	//	log.Println(err)
	//	return
	//}

	log.Printf("client ClientDialAndHandshake took %s", time.Since(start))

	// sending a million messages takes ~24seconds on _my_ Laptop
	printer := message.NewPrinter(language.German)
	start = time.Now()
	msgCount := 1_000_000
	if msgCount >= 50 {
		faktor := 0.00002252615
		if clientConfig.Encryption {
			faktor = 0.00002329565
		}

		log.Printf("expected time to send %s msgs is ~%ds", printer.Sprintf("%d", msgCount), int(math.Round(float64(msgCount)*faktor))+1)
		log.PauseLogging()
	}
	for i := 0; i < msgCount; i++ {
		m := fmt.Sprintf("Msg: %2d", i+1)
		log.Printf("client sending '%s'", m)
		c.Write(ipc.String, []byte(m))
	}
	log.ContinueLogging()
	log.Printf("client sending %s msgs took %s", printer.Sprintf("%d", msgCount), time.Since(start))

	log.Println("client sending <action>")
	c.Write(ipc.String, []byte(ipc.IntermediateActionMessage))

	firstTime := true
	for {
		if firstTime {
			firstTime = false
			log.Println("client waiting for first message...")
		} else {
			log.Println("client waiting for further messages....")
		}

		message, err := c.Read()
		if err == nil {
			log.Printf("client received Msg(%s): %s - Message type: %d", message.MsgType, string(message.Data), message.MsgType)
			internalReply := fmt.Sprintf("reply from example client: reply to '%s'", message.Data)
			c.Write(ipc.Custom, []byte(internalReply))
			log.Printf("client sent Msg: '%s'", internalReply)
			if string(message.Data) == "3.1415926535" {
				log.Printf("client received magic message, now closing...")
				time.Sleep(500 * time.Millisecond)
				break
			}
		} else {
			log.Println(err)
			break
		}
	}
	log.Println("example process finished ok.")
}

func server(ipcName string, serverConfig *ipc.ServerConfig, wait chan bool) {
	log.Printf("starting server '%s'.", ipcName)
	s, err := ipc.StartServer(ipcName, serverConfig)
	if err != nil {
		log.Println("server error", err)
		return
	}
	wait <- false

	log.Printf("server status: %s", s.Status())

	for {
		msg, err := s.Read()

		if err == nil {
			msgData := string(msg.Data)
			log.Printf("server received Msg: '%s' - Message type: %d", msgData, msg.MsgType)
			if msgData == ipc.IntermediateActionMessage {
				log.Printf("server received INTERMEDIATE '%s' ... reply to action with %f.", msgData, 3.1415926535)
				err := s.Write(ipc.Float, []byte("3.1415926535"))
				if err != nil {
					panic(err)
				}
			} else if msgData == ipc.FinalMessage {
				log.Printf("server received FINAL '%s' from  client", msgData)
				log.Println("server CLOSEs connection.")
				s.Close()
				return
			} else if msgData == ipc.InitialMessage {
				err := s.Write(ipc.Float, []byte("2.71828"))
				if err != nil {
					panic(err)
				}
			}
		} else {
			break
		}
	}
}
