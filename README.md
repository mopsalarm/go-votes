go-votes
========

To test, run a docker container with redis. It will store votes in `/tmp/votes`:
```
docker run --name redis -d -v /tmp/votes:/data redis:3.2 redis-server --appendonly yes
```

Then run `go-votes` using the docker image
```
docker run --rm mopsalarm/go-votes --redis=redis:6379
```

Or build it yourself using `./build.sh` and then start the binary
```
./go-votes --redis=redis-host:6379
```

You can also import a csv file. See `./go-votes --help` for more information.
```
./go-votes --redis=redis-host:6379 --import all-votes.csv
```

Api
===

Use like this:
```
// Write one vote and read it out again
$ curl -XPOST -d '{"action": 5, "thing": 1337}' localhost:8080/votes/15000
$ curl localhost:8080/votes/15000
{"duration":0,"nextSyncId":1,"votes":[5,1337]}

// Write another vote and read it
$ curl -XPOST -d '{"action": 7, "thing": 9000}' localhost:8080/votes/15000
$ curl localhost:8080/votes/15000
{"duration":0,"nextSyncId":2,"votes":[5,1337,7,9000]}

// Query with sync id to get only new stuff
$ curl "localhost:8080/votes/15000?syncId=1"
{"duration":0,"nextSyncId":2,"votes":[7,9000]}
```

