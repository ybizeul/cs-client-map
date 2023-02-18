package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// Command line arguments
var version = flag.Bool("v", false, "Print version and exits")
var api_endpoint = flag.String("e", "", "CloudSecure enpoint for the instance to use, i.e. 'psxxx.cs01.cloudinsights.netapp.com'. Can be set in CI_ENDPOINT environement variable too")
var api_key = flag.String("k", "", "API Key used to authenticate with CloudSecure service. Can be set in CI_API_KEY environement variable too")
var depth = flag.Int("p", 1, "Path depth to output")
var fromTime = flag.Int64("f", 0, "From time. Unix ms timestamp which defaults to yesterday at 00:00")
var toTime = flag.Int64("t", 0, "To time. Unix ms timestamp which defaults to today at 00:00")

// Constants
const VERSION = "0.9"
const WORKERS = 10
const LIMIT = 1000

var CACHE *cache.Cache

// Data types
type Activities struct {
	Count   int64      `json:"count"`
	Limit   int64      `json:"limit"`
	Offset  int64      `json:"offset"`
	Results []Activity `json:"results"`
}
type Activity struct {
	AccessLocation string `json:"accessLocation"`
	EntityPath     string `json:"entityPath"`
}

func yesterdayMidnightUnix() int64 {
	t := time.Now()
	y := t.Add(-time.Hour * 24)
	m := time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, time.Local)
	return m.UnixMicro()
}
func todayMidnightUnix() int64 {
	t := time.Now()
	m := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	return m.UnixMicro()
}

// Main
func main() {
	// Parse command line arguments
	flag.Parse()

	if *version == true {
		fmt.Printf("%s v%s (https://github.com/ybizeul/cs-client-map)\n", os.Args[0], VERSION)
		os.Exit(0)
	}
	if *fromTime == 0 {
		*fromTime = yesterdayMidnightUnix()
	}

	if *toTime == 0 {
		*toTime = yesterdayMidnightUnix()
	}

	if *api_endpoint == "" {
		*api_endpoint = os.Getenv("CI_ENDPOINT")
	}

	if *api_key == "" {
		*api_key = os.Getenv("CI_API_KEY")
	}

	// Initialize cache
	CACHE = cache.New(0, 0)

	// Fetch activities from Cloud Insights
	activities := fetchActivities(*fromTime, *toTime, 0)

	count := activities.Count
	jobsCount := int(math.Ceil(float64(count / LIMIT)))

	jobs := make(chan int, int(jobsCount))

	var wg sync.WaitGroup

	for w := 1; w <= WORKERS; w++ {
		wg.Add(1)
		go worker(w, jobs, &wg)
	}

	for job := 1; job <= jobsCount; job++ {
		jobs <- job
	}

	close(jobs)
	wg.Wait()

	items := CACHE.Items()
	for i := range items {
		fmt.Println(i)
	}
}

func worker(w int, jobs chan int, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		processJobs(w, job)
	}
}

func processJobs(w int, job int) {
	//fmt.Printf("Worker %d, Job %d\n", w, job)
	activities := fetchActivities(*fromTime, *toTime, job*LIMIT)
	// Display activities
	for _, a := range activities.Results {
		split := strings.Split(a.EntityPath, "/")
		split = split[0 : *depth+1]
		entry := fmt.Sprintf("%s\t%s", a.AccessLocation, strings.Join(split, "/"))
		CACHE.Add(entry, 1, 0)
	}
}

// fetchActivities for the given from and to time with the indicated offset
func fetchActivities(from int64, to int64, offset int) *Activities {
	client := &http.Client{}

	url := fmt.Sprintf("https://%s/rest/v1/cloudsecure/activities?from=%d&to=%d&offset=%d", *api_endpoint, from, to, offset)

	//fmt.Println(url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("X-CloudInsights-ApiKey", *api_key)

	resp, err := client.Do(req)

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var activities Activities

	json.Unmarshal(b, &activities)

	return &activities
}
