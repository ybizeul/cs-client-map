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
var api_endpoint = flag.String("e", "", "CloudSecure enpoint for the instance to use, i.e. 'psxxx.cs01.cloudinsights.netapp.com'. Can be set in CS_ENDPOINT environement variable too")
var api_key = flag.String("k", "", "API Key used to authenticate with CloudSecure service. Can be set in CS_API_KEY environement variable too")
var depth = flag.Int("p", 1, "Path depth to output")
var fromTime = flag.Int64("f", 0, "From time. Unix ms timestamp which defaults to yesterday at 00:00")
var toTime = flag.Int64("t", 0, "To time. Unix ms timestamp which defaults to today at 00:00")

// Constants
const VERSION = "0.9"
const WORKERS = 10
const LIMIT = 1000

// CACHE maintains a list of unique client / path string
var CACHE *cache.Cache

// STATUS maintains completion percentage of each thread
var STATUS []int

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
		*api_endpoint = os.Getenv("CS_ENDPOINT")
	}

	if *api_key == "" {
		*api_key = os.Getenv("CS_API_KEY")
	}

	// Check mendatory parameters
	if *api_endpoint == "" {
		fmt.Fprintf(os.Stderr, "Missing api endpoint as argument (-e) or CS_ENDPOINT env\n")
		os.Exit(1)
	}
	if *api_key == "" {
		fmt.Fprintf(os.Stderr, "Missing api key as argument (-k) or CS_API_KEY env\n")
		os.Exit(1)
	}

	// Initialize cache
	CACHE = cache.New(0, 0)

	// Initialize status
	STATUS = make([]int, WORKERS)

	// Fetch activities from Cloud Insights
	activities := fetchActivities(*fromTime, *toTime, 0)

	count := activities.Count

	// Log job size to StdErr
	fmt.Fprintf(os.Stderr, "Analyzing %d records\n", count)

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
	fmt.Fprintf(os.Stderr, "\n")
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
	for i := 0; i < len(activities.Results); i++ {
		a := activities.Results[i]
		split := strings.Split(a.EntityPath, "/")
		split = split[0 : *depth+1]
		entry := fmt.Sprintf("%s\t%s", a.AccessLocation, strings.Join(split, "/"))
		CACHE.Add(entry, 1, 0)
		STATUS[w-1] = int(100 * i / len(activities.Results))
		updateJobStatus()
	}
	STATUS[w-1] = 100
	updateJobStatus()
}

func updateJobStatus() {
	avg := 0
	for i := 0; i < len(STATUS); i++ {
		avg += STATUS[i]
	}
	percent := int(math.Floor(float64(avg) / float64(len(STATUS))))
	fmt.Fprintf(os.Stderr, "\rDone %d%%", percent)
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
