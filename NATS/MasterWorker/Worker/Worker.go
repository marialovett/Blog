package main

import (
	"os"
	"fmt"
	"github.com/nats-io/nats"
	"time"
	"github.com/cube2222/Blog/NATS/MasterWorker"
	"github.com/golang/protobuf/proto"
	"net/http"
	"bytes"
	"io/ioutil"
	"sort"
	"strings"
	"github.com/satori/go.uuid"
	"sync"
)

var nc *nats.Conn

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Wrong number of arguments. Need NATS server address.")
		return
	}

	var err error

	nc, err = nats.Connect(os.Args[1])
	if err != nil {
		fmt.Println(err)
	}

	for i := 0; i < 8; i++ {
		go doWork()
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}

func doWork() {
	for {
		msg, err := nc.Request("Work.TaskToDo", nil, 1 * time.Second)
		if err != nil {
			fmt.Println("Something went wrong. Waiting 2 seconds before retrying:", err)
			continue
		}

		curTask := Transport.Task{}
		err = proto.Unmarshal(msg.Data, &curTask)
		if err != nil {
			fmt.Println("Something went wrong. Waiting 2 seconds before retrying:", err)
			continue
		}

		msg, err = nc.Request("Discovery.FileServer", nil, 1000 * time.Millisecond)
		if err != nil {
			fmt.Println("Something went wrong. Waiting 2 seconds before retrying:", err)
			continue
		}

		fileServerAddressTransport := Transport.DiscoverableServiceTransport{}
		err = proto.Unmarshal(msg.Data, &fileServerAddressTransport)
		if err != nil {
			fmt.Println("Something went wrong. Waiting 2 seconds before retrying:", err)
			continue
		}

		fileServerAddress := fileServerAddressTransport.Address
		r, err := http.Get(fileServerAddress + "/" + curTask.Uuid)
		if err != nil {
			fmt.Println("Something went wrong. Waiting 2 seconds before retrying:", err)
			continue
		}

		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Println("Something went wrong. Waiting 2 seconds before retrying:", err)
			continue
		}

		words := strings.Split(string(data), ",")
		sort.Strings(words)
		wordCounts := make(map[string]int)
		for i := 0; i < len(words); i++{
			wordCounts[words[i]] = wordCounts[words[i]] + 1
		}

		resultData := make([]byte, 0, 1024)
		buf := bytes.NewBuffer(resultData)

		for key, value := range wordCounts {
			fmt.Fprintln(buf, key, ":", value)
		}

		curTask.Finisheduuid = uuid.NewV4().String()
		r, err = http.Post(fileServerAddress + "/" + curTask.Finisheduuid, "", buf)
		if err != nil || r.StatusCode != http.StatusOK {
			fmt.Println("Something went wrong. Waiting 2 seconds before retrying:", err, ":", r.StatusCode)
			continue
		}

		data, err = proto.Marshal(&curTask)
		if err != nil {
			fmt.Println("Something went wrong. Waiting 2 seconds before retrying:", err)
			continue
		}

		nc.Publish("Work.TaskFinished", data)
	}
}

