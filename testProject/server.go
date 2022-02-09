package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

func init() {
	log.Println(strings.Title("hello"))
	log.SetPrefix("[")
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

type Client struct {
	C    chan string //用于发送数据的管道
	Name string      //用户名
	Add  string      //地址
}

//在线用户 cliAddr --->Client
var onlineMap map[string]*Client
var message chan string = make(chan string)

// 处理用户连接|主动退出和超时机制
func HandleConn(conn net.Conn) {

	defer conn.Close()
	defer fmt.Println("HandleConn 协程退出")
	//	获取客户端的网络地址
	cliAddr := conn.RemoteAddr().String()

	cli := &Client{make(chan string), cliAddr, cliAddr}
	//	把结构体添加入map
	onlineMap[cliAddr] = cli
	//	广播某个人在线
	message <- MakeMsg(cli, "login")
	//新开一个协程，给当前客户端发送信息
	go WritrMsgToClient(cli, conn)

	log.Println(MakeMsg(cli, "login"))

	//是否主动退出，使用管道控制退出
	isQuit := make(chan uint)
	//是否有用户输入|定义一个空结构体 的chan
	hasUserInput := make(chan uint)

	//新建一个协程，接受用户发送来的数据
	go func() {
		defer log.Println("接受用户发送来的数据 处理程序退出")
		buf := make([]byte, 2048)
		for {
			n, err := conn.Read(buf)
			//log.Println(n, "错误是:", err)
			if n == 0 || nil == io.EOF { //对方断开 或者其他
				fmt.Println(err)
				isQuit <- 1
				return
			}

			msg := buf[:n]
			if string(msg) == "who" {
				var buffer bytes.Buffer
				for _, tmp := range onlineMap {
					buffer.WriteString(tmp.Name)
					buffer.WriteString(" ")
				}
				conn.Write(buffer.Bytes())
			} else {
				//转发内容
				message <- MakeMsg(cli, string(msg))
			}
			hasUserInput <- 1
		}
	}()

	for {
		//循环执行，避免连接断开
		select {
		case <-isQuit:
			delete(onlineMap, cliAddr)
			message <- MakeMsg(cli, cliAddr+"login out")
			return
		case <-hasUserInput:

		case <-time.After(time.Second * 10):
			delete(onlineMap, cliAddr)
			message <- MakeMsg(cli, " time out leave")
			return
		}
	}
}

func MakeMsg(cli *Client, msg string) (buf string) {
	buf = fmt.Sprint("{", cli.Add, "}", cli.Name, ": ", msg)
	return
}

//新开一个协程，给当前客户端发送信息
func WritrMsgToClient(cli *Client, conn net.Conn) {
	for msg := range cli.C {
		conn.Write([]byte(msg + "\n"))
	}
}

//新开一个协程，转发消息|只要有消息来了，遍历map，给每个成员发送消息
func ManagerMsg() {
	for {
		msg := <-message //没有消息前会阻塞
		//遍历map，给每个成员发送
		for _, cli := range onlineMap {
			//包括转发给自己
			cli.C <- msg
		}
	}
}

func main() {
	log.Println("服务器监听9050")
	listen, err := net.Listen("tcp", "0.0.0.0:9050")
	if err != nil {
		log.Println("net listen err", err)
		return
	}
	defer listen.Close()
	//新开一个协程，转发消息|只要有消息来了，遍历map，给每个成员发送消息
	go ManagerMsg()
	//给map分派空间
	onlineMap = make(map[string]*Client)
	//	主协程，等待客户连接
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		//处理用户连接
		go HandleConn(conn)
	}

}
