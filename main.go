package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/redis.v3"
)

// Write one vote and read it out again
// $ curl -XPOST -d '{"action": 5, "thing": 1337}' localhost:8080/votes/15000
// $ curl localhost:8080/votes/15000
// {"duration":0,"nextSyncId":1,"votes":[5,1337]}

// Write another vote and read it
// $ curl -XPOST -d '{"action": 7, "thing": 9000}' localhost:8080/votes/15000
// $ curl localhost:8080/votes/15000
// {"duration":0,"nextSyncId":2,"votes":[5,1337,7,9000]}

// Query with sync id to get only new stuff
// $ curl "localhost:8080/votes/15000?syncId=1"
// {"duration":0,"nextSyncId":2,"votes":[7,9000]}

func main() {
	flagImport := flag.String("import", "", "A csv file to import. Each record must have 3 fields: userId,actionId,thingId")
	flagRedis := flag.String("redis", "localhost:6379", "Address of the redis server")
	flag.Parse()

	client := redis.NewClient(&redis.Options{Addr: *flagRedis})
	defer client.Close()

	// check if redis is available an currently ready to use
	_, err := client.Ping().Result()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// import csv file
	if *flagImport != "" {
		importCsvFile(client, *flagImport)
		return
	}

	// start webserver
	r := gin.Default()
	r.GET("/votes/:user", func(c *gin.Context) {
		getVotesForUser(client, c)
	})

	r.POST("/votes/:user", func(c *gin.Context) {
		storeVote(client, c)
	})

	r.Run(":8080")
}

type Vote struct {
	Action uint8
	Thing  uint32
}

func storeVote(client *redis.Client, c *gin.Context) {
	var vote Vote
	if err := c.BindJSON(&vote); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userId := c.Param("user")
	voteValue := int64(vote.Thing)*10 + int64(vote.Action)

	key := "user:" + userId + ":votes"
	client.RPush(key, strconv.FormatInt(voteValue, 10))
	c.Status(http.StatusNoContent)
}

func getVotesForUser(client *redis.Client, c *gin.Context) {
	startTime := time.Now()

	// validate startId
	startAt, err := strconv.ParseInt(c.DefaultQuery("syncId", "0"), 10, 64)
	if err != nil || startAt < 0 {
		startAt = 0
	}

	// get elements from redis.
	key := "user:" + c.Param("user") + ":votes"
	result, err := client.LRange(key, startAt, -1).Result()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// convert back to action/thing-format
	votes := make([]uint32, 2*len(result))
	for idx, val := range result {
		parsed, _ := strconv.ParseInt(val, 10, 32)

		votes[2*idx] = uint32(parsed % 10)
		votes[2*idx+1] = uint32(parsed) / 10
	}

	// calculate next sync id as the current length of the vote log.
	nextSyncId := startAt + int64(len(result))

	c.JSON(http.StatusOK, gin.H{
		"votes":      votes,
		"duration":   time.Since(startTime) / time.Millisecond,
		"nextSyncId": nextSyncId,
	})
}

func importCsvFile(client *redis.Client, csvFile string) {
	count := 0
	fp, err := os.Open(csvFile)
	if err != nil {
		fmt.Println("Could not open csv file", err)
		os.Exit(1)
	}

	defer fp.Close()

	reader := csv.NewReader(fp)
	reader.FieldsPerRecord = 3
	for {
		count += 1
		record, err := reader.Read()
		if record == nil {
			break
		}

		if err != nil {
			log.Print(err)
			continue
		}

		userId := record[0]
		actionId, _ := strconv.ParseInt(record[1], 10, 64)
		itemId, _ := strconv.ParseInt(record[2], 10, 64)

		voteValue := itemId*10 + actionId

		key := "user:" + userId + ":votes"
		client.RPush(key, strconv.FormatInt(voteValue, 10))

		if count%100000 == 0 {
			fmt.Println(count)
		}
	}
}
