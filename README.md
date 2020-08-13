# reproduce-reload-hangs

# Requirements

You need a working redis connection

#  1

Open a terminal session and be on the root of this repo. start the main service

```
go run main.go
```

This starts a service that simulate dashboard. And the gateway  loads policies from this service.

# 2

On another terminal session

```
cd run && tyk --conf tyk.json
```

 We are starting tyk in the `run` directory with the configuration `tyk.json`
tyk should be version `2.9.4`

# 3

Run the simulation

```
go run client/main.go
```

On the simulation we
- trigger a reload
- creates a key with a policy that will be available by the trigger above. We retry with exponential backoff untill this succeds and record total time taken. This is the rough time taken between the policy trigger and the policy being reloaded.

We do this concurrently with about `500` request (with the two seps) a second

This summeriest the time taken for the 10 highest number of policies.

On my observation

on `v2.9.4`
```
SUCCESS 82904 ==>21.805158857s
SUCCESS 82903 ==>21.150249165s
SUCCESS 82902 ==>21.377872548s
SUCCESS 82901 ==>23.754652254s
SUCCESS 82900 ==>20.814489394s
SUCCESS 82899 ==>21.235413466s
SUCCESS 82898 ==>21.752531061s
SUCCESS 82897 ==>22.380861776s
SUCCESS 82896 ==>22.39114931s
SUCCESS 82895 ==>24.977897559s
```

Note `82904` is the number of policies loaded and `21.805158857s` is the time taken to trigger the reload
Running this a couple of times it will block/hang and you wont get to seethe summary at all.

And with the changed PR

```
SUCCESS 82286 ==>5.667192695s
SUCCESS 82285 ==>4.161559012s
SUCCESS 82284 ==>4.041969781s
SUCCESS 82283 ==>3.99249288s
SUCCESS 82282 ==>4.064978045s
SUCCESS 82281 ==>4.168690634s
SUCCESS 82280 ==>4.216894293s
SUCCESS 82279 ==>4.059365489s
SUCCESS 82278 ==>3.998513712s
SUCCESS 82277 ==>3.878652382s
```

Also this extremely unlikely scenario. But highlights the long time taken to load policies.
Also you can run this as many times a s you want, you will still get similar results.