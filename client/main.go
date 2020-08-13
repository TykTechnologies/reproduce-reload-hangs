package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
)

var client = &http.Client{
	Transport: &http.Transport{
		MaxConnsPerHost: 2000,
	},
}

var origin string

func main() {
	flag.Parse()
	pid := flag.Arg(0)
	i, _ := strconv.Atoi(pid)
	p, err := os.FindProcess(i)
	if err != nil {
		log.Fatal(err)
	}
	defer p.Signal(syscall.SIGTERM)
	origin = getOrigin()
	var wg sync.WaitGroup
	r := &report{
		success: make(map[int][]time.Duration),
		fail:    make(map[int][]time.Duration),
		index:   make(map[int]string),
	}
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	n := 0
	x := 0
	total := 0
end:
	for {
		select {
		case <-timer.C:
			total += n
			if x == 4 {
				break end
			}
			n = 0
			x++
		default:
			if n > 500 {
				continue
			}
			n++
			wg.Add(1)
			go run(&wg, r)
		}
	}
	fmt.Println("Total ", total, "...waiting")
	wg.Wait()
	r.report()
}

type report struct {
	success map[int][]time.Duration
	fail    map[int][]time.Duration
	index   map[int]string
	mu      sync.Mutex
}

func (r *report) ok(id int, dur time.Duration) {
	r.mu.Lock()
	r.success[id] = append(r.success[id], dur)
	r.mu.Unlock()
}

func (r *report) died(id int, key string, dur time.Duration) {
	r.mu.Lock()
	r.fail[id] = append(r.fail[id], dur)
	r.index[id] = key
	r.mu.Unlock()
}

type card struct {
	index int
	dur   time.Duration
}

func reportCard(s map[int][]time.Duration) []*card {
	if len(s) == 0 {
		return nil
	}
	var cards []*card
	for k, v := range s {
		sort.Slice(v, func(i, j int) bool {
			return v[i] > v[j]
		})
		cards = append(cards, &card{
			index: k,
			dur:   v[0],
		})
	}
	sort.Slice(cards, func(i, j int) bool {
		return cards[i].index > cards[j].index
	})
	if len(cards) < 10 {
		return cards
	}
	return cards[:10]
}

func (r *report) report() {
	for _, v := range reportCard(r.success) {
		fmt.Printf("SUCCESS %d ==>%v\n", v.index, v.dur)
	}
	fmt.Println("============================>")
	for _, v := range reportCard(r.fail) {
		fmt.Printf("FAIL %d ==>%v\n", v.index, v.dur)
	}
}

func getOrigin() string {
	res, err := client.Get("http://localhost:9000/origin")
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	n := map[string]interface{}{
		"apply_policies": []string{
			string(b),
		},
	}
	x, _ := json.Marshal(n)
	return string(x)
}

func run(wg *sync.WaitGroup, r *report) {
	start := time.Now()
	var index int
	var k string
	var err error
	var res *http.Response
	defer func() {
		dur := time.Since(start)
		if err == nil {
			r.ok(index, dur)
		} else {
			r.died(index, k, dur)
		}
		wg.Done()
	}()

	res, err = client.Get("http://localhost:9000/create")
	if err != nil {
		return
	}
	var buf bytes.Buffer
	io.Copy(&buf, res.Body)
	parts := strings.Split(buf.String(), "|")
	index, _ = strconv.Atoi(parts[1])
	k = parts[0]
	err = key(parts[0])
}

func key(p string) error {
	n := map[string]interface{}{
		"apply_policies": []string{
			p,
		},
	}
	b, _ := json.Marshal(n)
	return backoff.Retry(func() error {
		r, _ := http.NewRequest(http.MethodPost, "http://localhost:7000/tyk/keys/create", bytes.NewReader(b))
		r.Header.Add("X-Tyk-Authorization", "352d20ee67be67f6340b4c0605b044b7")
		res, err := client.Do(r)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return errors.New("bad request")
		}
		return nil
	}, backoff.NewExponentialBackOff())
}

func checkOrigin() int {
	r, _ := http.NewRequest(http.MethodPost, "http://localhost:7000/tyk/keys/create", strings.NewReader(origin))
	r.Header.Add("X-Tyk-Authorization", "352d20ee67be67f6340b4c0605b044b7")
	res, err := client.Do(r)
	if err != nil {
		return http.StatusInternalServerError
	}
	defer res.Body.Close()
	return res.StatusCode
}
