package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/go-redis/redis"
	"github.com/google/uuid"
)

type List struct {
	Message []DBPolicy
	Nonce   string
}

var client = &http.Client{
	Transport: &http.Transport{
		MaxConnsPerHost: 400,
	},
}

var (
	RedisPubSubChannel  = "tyk.cluster.notifications"
	NoticePolicyChanged = "PolicyChanged"
)

type Notification struct {
	Command   string `json:"command"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		PoolSize: 10,
		Password: "", // add password
	})
	fmt.Println(rdb.Ping().Result())
	note, _ := json.Marshal(&Notification{
		Command: NoticePolicyChanged,
	})
	notify := string(note)
	n := 100000
	msg := make([]DBPolicy, n)
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		msg[i] = DBPolicy{
			ID:    uuid.New(),
			Name:  "policy-name-" + strconv.Itoa(i),
			OrgID: "monarch",
		}
	}
	origin := msg[0].ID.String()
	def := 80400
	index := def
	nonce := "nonce"
	serve := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		ls := List{
			Message: msg[:index],
			Nonce:   nonce,
		}
		json.NewEncoder(w).Encode(ls)
		fmt.Println("======> ", len(ls.Message))
	}
	create := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if index >= n {
			index = def
		}
		id := msg[index].ID.String() + "|" + strconv.Itoa(index)
		index++
		err := rdb.Publish(RedisPubSubChannel, notify).Err()
		if err != nil {
			fmt.Println("===> Publishin notification error", err)
		}
		fmt.Println(index)
		w.Write([]byte(id))
	}
	srv := http.NewServeMux()
	srv.HandleFunc("/system/policies", serve)
	srv.HandleFunc("/create", create)
	srv.HandleFunc("/origin", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(origin))
	})
	http.ListenAndServe(":9000", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		srv.ServeHTTP(w, r)
	}))
}

func reload() {
	r, _ := http.NewRequest(http.MethodGet, "http://localhost:7000/tyk/reload", nil)
	r.Header.Add("X-Tyk-Authorization", "352d20ee67be67f6340b4c0605b044b7")
	res, err := client.Do(r)
	if err != nil {
		log.Print("=== ERROR ", err)
		return
	}
	defer res.Body.Close()
}

type DBPolicy struct {
	ID    uuid.UUID `bson:"id,omitempty" json:"id"`
	Name  string    `bson:"name" json:"name"`
	OrgID string    `bson:"org_id" json:"org_id"`
	// AccessRights map[string]DBAccessDefinition `bson:"access_rights" json:"access_rights"`
}

type DBAccessDefinition struct {
	APIName  string   `json:"apiname"`
	APIID    string   `json:"apiid"`
	Versions []string `json:"versions"`
}
