package main

import (
	"fmt"
	ipc "github.com/igadmg/golang-ipc"
	log "github.com/igadmg/golang-ipc/ipclogging"
	"time"
)

func main() {
	log.DoDebug = true
	go server()
	time.Sleep(50_000 * time.Nanosecond) // client shouldn't connect before server listens

	clientConfig := &ipc.ClientConfig{
		SocketBasePath: ipc.DefaultClientConfig.SocketBasePath,
		Timeout:        ipc.DefaultClientConfig.Timeout,
		RetryTimer:     ipc.DefaultClientConfig.RetryTimer,
		MaxMsgSize:     ipc.DefaultClientConfig.MaxMsgSize,
		Encryption:     true,
	}
	c, err := ipc.DialAndHandshake("example1", clientConfig)
	if err != nil {
		log.Println(err)
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

	for i := 0; i < 10; i++ {
		m := fmt.Sprintf("Msg: %2d", i+1)
		log.Printf("client sending '%s'", m)
		c.Write(ipc.String, []byte(m))
	}
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
			if message.MsgType < 0 {
				log.Printf("client received IpcInternal: %s", message.IpcType.String())

				if message.Status == ipc.ClientReConnecting {
					c.Close()
					return
				}
			} else {
				log.Printf("client received Msg(%s): %s - Message type: %d", message.MsgType, string(message.Data), message.MsgType)
				internalReply := fmt.Sprintf("reply from example client: reply to '%s'", message.Data)
				c.Write(ipc.Custom, []byte(internalReply))
				log.Printf("client sent Msg: '%s'", internalReply)
				if string(message.Data) == "3.1415926535" {
					log.Printf("client received magic message, now closing...")
					time.Sleep(500 * time.Millisecond)
					break
				}
			}
		} else {
			log.Println(err)
			break
		}
	}
	log.Println("example process finished ok.")
}

func server() {
	serverConfig := &ipc.ServerConfig{
		SocketBasePath:    ipc.DefaultServerConfig.SocketBasePath,
		Timeout:           ipc.DefaultServerConfig.Timeout,
		MaxMsgSize:        ipc.DefaultServerConfig.MaxMsgSize,
		Encryption:        true,
		UnmaskPermissions: true, // true will make the socket writable for any user
	}
	log.Println("example is starting server.")
	s, err := ipc.StartServer("example1", serverConfig)
	if err != nil {
		log.Println("server error", err)
		return
	}

	log.Printf("server status: %s", s.Status())

	for {
		msg, err := s.Read()

		if err == nil {
			if msg.MsgType < 0 {
				if msg.Status == ipc.ServerConnected {
					log.Printf("server received IpcInternal: %s", s.Status())
					internalReply := "server reply: server is connected to client"
					s.Write(ipc.String, []byte(internalReply))
					log.Printf("server sent Msg: '%s'", internalReply)
				} else {
					log.Printf("server received IpcInternal: %s", string(msg.Data))
				}
			} else {
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
			}
		} else {
			break
		}
	}
}
