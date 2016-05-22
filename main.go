package main

import (
	"encoding/csv"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"errors"
	"io"

	"github.com/gin-gonic/gin"
	"gopkg.in/cheggaaa/pb.v1"
	"gopkg.in/redis.v3"
)

func main() {
	flagImport := flag.String("import", "", "A csv file to import. Each record must have 3 fields: userId,actionId,thingId")
	flagRedis := flag.String("redis", "localhost:6379", "Address of the redis server")
	flagListen := flag.String("listen", "0.0.0.0:8080", "Listen address for the server")
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

	r.Run(*flagListen)
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

	if vote.Action&0xf != vote.Action {
		c.AbortWithError(http.StatusBadRequest, errors.New("Action is invalid"))
		return
	}

	userId := c.Param("user")
	voteValue := int64(vote.Thing)<<4 + int64(vote.Action&0xf)

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

		votes[2*idx] = uint32(parsed & 0xf)
		votes[2*idx+1] = uint32(parsed) >> 4
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
		log.Fatal("Could not open csv file", err)
		return
	}

	defer fp.Close()

	var reader io.Reader = fp
	if info, err := fp.Stat(); err == nil {
		bar := pb.New64(info.Size())
		bar.Units = pb.U_BYTES
		bar.RefreshRate = 1 * time.Second
		bar.Start()

		defer bar.Finish()

		reader = bar.NewProxyReader(reader)
	}

	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = 3
	for {
		record, err := csvReader.Read()
		if record == nil {
			break
		}

		if err != nil {
			log.Print(err)
			continue
		}

		userId := record[0]
		actionId, err := strconv.ParseInt(record[1], 10, 64)
		if err != nil {
			log.Fatal("Could not parse action id", actionId)
		}

		itemId, err := strconv.ParseInt(record[2], 10, 64)
		if err != nil {
			log.Fatal("Could not parse item id", actionId)
		}

		if actionId&0xf != actionId {
			log.Fatal("Invalid action id:", actionId)
		}

		voteValue := (itemId << 4) | (actionId & 0xf)

		key := "user:" + userId + ":votes"
		client.RPush(key, strconv.FormatInt(voteValue, 10))

		count += 1
	}

	log.Println("Number of votes read:", count)
}
