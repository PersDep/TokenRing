package main

import (
	"./message"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const myConst = 4096

func runTokenRing(curNode, nodesAmount int, quitChannel chan struct{}, holdingTime time.Duration, needInit bool) {
	basePort, baseServicePort := 30000, 40000
	leftNode, rightNode := (curNode-1+nodesAmount)%nodesAmount, (curNode+1)%nodesAmount
	curPort := basePort + curNode
	servicePort := baseServicePort + curNode

	udp := "udp"
	curAddress, _ := net.ResolveUDPAddr(udp, "127.0.0.1:"+strconv.Itoa(curPort))
	connection, _ := net.ListenUDP(udp, curAddress)
	rightAddress, _ := net.ResolveUDPAddr(udp, "127.0.0.1:"+strconv.Itoa(basePort+rightNode))
	if needInit {
		dataMessage := string(message.Message{Type: "empty", Receiver: 1, Data: ""}.ConvertToJsonMsg())
		connection.WriteToUDP(message.Message{Type: "token", Receiver: -1, Data: dataMessage}.ConvertToJsonMsg(), rightAddress)
	}

	dataChannel := make(chan message.Message)
	timeoutChannel := make(chan time.Duration, 1)
	timeoutChannel <- holdingTime * time.Duration(nodesAmount) * 2
	go manageConnection(connection, dataChannel, timeoutChannel)

	serviceAddress, _ := net.ResolveUDPAddr(udp, "127.0.0.1:"+strconv.Itoa(servicePort))
	serviceConnection, _ := net.ListenUDP(udp, serviceAddress)
	serviceChannel := make(chan message.Message)
	timeoutServiceChannel := make(chan time.Duration, 1)
	timeoutServiceChannel <- 0
	go manageConnection(serviceConnection, serviceChannel, timeoutServiceChannel)

	leftAddress, _ := net.ResolveUDPAddr(udp, "127.0.0.1:"+strconv.Itoa(basePort+leftNode))
	taskChannel := make(chan message.Message, myConst)
	var msg message.Message
	madeChoice := false
	needDrop := false
	noToken := false
	doubleElection := make([]bool, nodesAmount)
	for i := range doubleElection {
		doubleElection[i] = false
	}
	for {
		select {
		case msg = <-dataChannel:
			{
			}
		case msg = <-serviceChannel:
			{
			}
		}
		if msg.Type == "token" {
			if noToken {
				fmt.Println(strconv.Itoa(curNode) + " discarded token")
			} else {
				if msg.Receiver == curNode {
					data := message.ConvertFromJsonMsg([]byte(msg.Data))
					if data.Type == "conf" {
						fmt.Println("node " + strconv.Itoa(curNode) + ": received token from node " + strconv.Itoa(leftNode) +
							" with delivery confirmation from node " + strconv.Itoa(data.Sender) + ", sending token to node " +
							strconv.Itoa(rightNode))
						msg = message.Message{Type: "token", Receiver: -1,
							Data: string(message.Message{Type: "empty", Receiver: -1, Data: ""}.ConvertToJsonMsg())}

					} else {
						fmt.Println("node " + strconv.Itoa(curNode) + ": received token from node " + strconv.Itoa(leftNode) +
							" with data from node " + strconv.Itoa(data.Sender) + " (data =`" + data.Data + "`), " +
							"sending token to node " + strconv.Itoa(rightNode))
						msg = message.Message{Type: "token", Receiver: data.Sender,
							Data: string(message.Message{Type: "conf", Receiver: data.Sender, Sender: curNode, Data: ""}.ConvertToJsonMsg())}
					}
				} else if msg.Receiver == -1 {
					select {
					case task := <-taskChannel:
						if task.Type == "send" {
							msg = message.Message{Type: "token", Receiver: task.Receiver,
								Data: string(message.Message{Type: "send", Receiver: task.Receiver, Sender: curNode, Data: task.Data}.ConvertToJsonMsg())}
						} else {
							needDrop = true
						}
					default:
						{
						}
					}
					fmt.Println("node " + strconv.Itoa(curNode) + ": received token from node " +
						strconv.Itoa(leftNode) + ", sending token to node " + strconv.Itoa(rightNode))
				} else {
					fmt.Println("node " + strconv.Itoa(curNode) + ": received token from node " + strconv.Itoa(leftNode) +
						", sending token to node " + strconv.Itoa(rightNode))
				}
				if !needDrop {
					send(msg, connection, rightAddress, holdingTime)
				}
				needDrop = false
			}
		} else if msg.Type == "timeout" {
			fmt.Println("node " + strconv.Itoa(curNode) + ": received timeout, starting election")
			noToken = true

			electionMessage := message.Message{Type: "elect", Receiver: rightNode, Sender: curNode, Data: strconv.Itoa(curNode)}
			msg := message.Message{Type: "elect", Receiver: curNode, Data: string(electionMessage.ConvertToJsonMsg())}
			send(msg, connection, rightAddress, holdingTime)

			electionMessage = message.Message{Type: "elect", Receiver: leftNode, Sender: curNode, Data: strconv.Itoa(curNode)}
			msg = message.Message{Type: "elect", Receiver: curNode, Data: string(electionMessage.ConvertToJsonMsg())}
			send(msg, connection, leftAddress, holdingTime)

			for i := range doubleElection {
				doubleElection[i] = false
			}
			doubleElection[curNode] = true
			madeChoice = false
		} else if msg.Type == "elect" {
			election(curNode, leftNode, rightNode, nodesAmount, connection, leftAddress, rightAddress,
				msg, doubleElection, &noToken, &madeChoice, holdingTime)
		} else if msg.Type == "makeChoice" {
			if noToken {
				fmt.Println("node "+strconv.Itoa(curNode)+": received election token with data:", msg)
				if msg.Receiver != curNode {
					send(msg, connection, rightAddress, holdingTime)
				}
				if chooseCandidate(message.ConvertFromJsonMsg(([]byte)(msg.Data)).Data) == curNode && !madeChoice {
					fmt.Println("node " + strconv.Itoa(curNode) + ": madeChoice token")
					msg = message.Message{Type: "token", Receiver: -1,
						Data: string(message.Message{Type: "empty", Receiver: -1, Data: ""}.ConvertToJsonMsg())}
					send(msg, connection, rightAddress, holdingTime)
					madeChoice = true
				}
				noToken = false
			}
		} else {
			fmt.Println("node "+strconv.Itoa(curNode)+": received service msg:", string(msg.ConvertToJsonMsg()))
			if msg.Type == "send" {
				taskChannel <- msg
			} else {
				taskChannel <- msg
			}
		}
	}
}

func manageConnection(Conn *net.UDPConn, dataChannel chan message.Message, timeoutChannel chan time.Duration) {
	var buf = make([]byte, myConst)
	var msg message.Message

	timeout := time.Duration(0)
	for {
		select {
		case timeout = <-timeoutChannel:
			{
			}
		default:
			{
				if timeout != 0 {
					Conn.SetReadDeadline(time.Now().Add(timeout))
				}
				messageLength, _, error := Conn.ReadFromUDP(buf)
				if error != nil {
					err, ok := error.(net.Error)
					if !ok || !err.Timeout() {
						panic(err)
					}
					msg = message.Message{Type: "timeout", Receiver: 0, Data: ""}
				} else {
					msg = message.ConvertFromJsonMsg(buf[0:messageLength])
				}

				dataChannel <- msg
			}
		}
	}
}

func send(msg message.Message, connection *net.UDPConn, address *net.UDPAddr, holdingTime time.Duration) {
	time.Sleep(holdingTime)
	connection.WriteToUDP(msg.ConvertToJsonMsg(), address)
}

func election(curNode, leftNode, rightNode, nodesAmount int, connection *net.UDPConn, leftAddress, rightAddress *net.UDPAddr,
	msg message.Message, doubleElection []bool, noToken, madeChoice *bool, holdingTime time.Duration) {

	if msg.Receiver == curNode {
		msg = message.Message{Type: "makeChoice", Receiver: curNode, Data: msg.Data}
		send(msg, connection, rightAddress, holdingTime)
	} else {
		fmt.Println("node "+strconv.Itoa(curNode)+": received election token with data:", msg)
		if !*madeChoice {
			*noToken = true
		}
		if !doubleElection[msg.Receiver] {
			temp := message.ConvertFromJsonMsg([]byte(msg.Data))
			temp.Sender, temp.Receiver = curNode, rightNode
			updateCandidates(&temp.Data, strconv.Itoa(curNode))
			msg = message.Message{Type: "elect", Receiver: msg.Receiver, Data: string(temp.ConvertToJsonMsg())}
			send(msg, connection, rightAddress, holdingTime)

			temp.Receiver = leftNode
			msg = message.Message{Type: "elect", Receiver: msg.Receiver, Data: string(temp.ConvertToJsonMsg())}
			send(msg, connection, leftAddress, holdingTime)

			doubleElection[msg.Receiver] = true
		} else {
			temp := message.ConvertFromJsonMsg([]byte(msg.Data))

			if temp.Sender == leftNode {
				temp.Sender = curNode
				temp.Receiver = rightNode
				updateCandidates(&temp.Data, strconv.Itoa(curNode))
				msg = message.Message{Type: "elect", Receiver: msg.Receiver, Data: string(temp.ConvertToJsonMsg())}
				send(msg, connection, rightAddress, holdingTime)
			} else {
				temp.Sender = curNode
				temp.Receiver = leftNode
				updateCandidates(&temp.Data, strconv.Itoa(curNode))
				msg = message.Message{Type: "elect", Receiver: msg.Receiver, Data: string(temp.ConvertToJsonMsg())}
				send(msg, connection, leftAddress, holdingTime)
			}
		}
	}
}

func updateCandidates(list *string, id string) {
	candidates := strings.Split(*list, " or ")

	for _, candidate := range candidates {
		if candidate == id {
			return
		}
	}
	*list = *list + " or " + id
}

func chooseCandidate(list string) int {
	candidates := strings.Split(list, " or ")
	max := -10

	for _, candidate := range candidates {
		val, _ := strconv.Atoi(candidate)
		if val > max {
			max = val
		}
	}

	return max
}
