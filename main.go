package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"gopkg.in/yaml.v2"
)

// Command line arguments
var version = flag.Bool("v", false, "Print version and exits")
var config_file = flag.String("c", "./config.yaml", "Configuration file path")
var api_endpoint = flag.String("e", "", "CloudSecure enpoint for the instance to use, i.e. 'psxxx.cs01.cloudinsights.netapp.com'. Can be set in CS_ENDPOINT environement variable too")
var api_key = flag.String("k", "", "API Key used to authenticate with CloudSecure service. Can be set in CS_API_KEY environement variable too")
var WORKERS = flag.Int("w", 10, "Number of concurrent workers")
var depth = flag.Int("p", 1, "Path depth to output")
var fromTime = flag.Int64("f", 0, "From time. Unix ms timestamp which defaults to yesterday at 00:00")
var toTime = flag.Int64("t", 0, "To time. Unix ms timestamp which defaults to today at 00:00")

// Constants
const VERSION = "0.8.2"

// Limit parameter to be sent to CloudSecure API, at the current time, maximum
// in 1000
const LIMIT = 1000

// CACHE maintains a list of unique client / path string
var CACHE *cache.Cache

// COUNT is the total number of records to fetch
var COUNT int64

// STATUS maintains completion percentage of each thread
var STATUS []int

// Configuration file struct
var CONFIG Config

// HTTP_RETRIES set the number of time a request should be retried before failing
const HTTP_RETRIES = 3

// Data types
type Config struct {
	CS_API_KEY      string
	CS_API_ENDPOINT string
}

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

// Initialize function
func initialize() {
	// fromTime
	if *fromTime == 0 {
		*fromTime = yesterdayMidnightUnix()
	}

	if *toTime == 0 {
		*toTime = todayMidnightUnix()
	}

	// Read configuration file
	if *config_file == "" {
		if _, err := os.Stat("./config.yaml"); err == nil {
			*config_file = "./config.yaml"
		}
	}

	if *config_file != "" {
		yamlFile, err := ioutil.ReadFile(*config_file)
		if err != nil {
			log.Printf("yamlFile.Get err   #%v ", err)
		}
		err = yaml.Unmarshal(yamlFile, &CONFIG)
		if err != nil {
			log.Fatalf("Unmarshal: %v", err)
		}
	}

	// Read API enpoint parameter
	if *api_endpoint == "" {
		*api_endpoint = os.Getenv("CS_API_ENDPOINT")
	}
	if *api_endpoint == "" {
		*api_endpoint = CONFIG.CS_API_ENDPOINT
	}
	if *api_endpoint == "" {
		fmt.Fprintf(os.Stderr, "Missing api endpoint as argument (-e) or CS_API_ENDPOINT env\n")
		os.Exit(1)
	}

	// Read API key parameter
	if *api_key == "" {
		*api_key = os.Getenv("CS_API_KEY")
	}
	if *api_key == "" {
		*api_key = CONFIG.CS_API_KEY
	}
	if *api_key == "" {
		fmt.Fprintf(os.Stderr, "Missing api key as argument (-k) or CS_API_KEY env\n")
		os.Exit(1)
	}

	// Initialize cache
	CACHE = cache.New(0, 0)

	// Initialize status
	STATUS = make([]int, *WORKERS)
}

// Main
func main() {
	// Parse command line arguments
	flag.Parse()

	if *version == true {
		fmt.Printf("%s v%s (https://github.com/ybizeul/cs-client-map)\n", os.Args[0], VERSION)
		os.Exit(0)
	}

	initialize()

	// Fetch activities from Cloud Insights
	activities := fetchActivities(*fromTime, *toTime, 0)

	COUNT = activities.Count

	f := time.UnixMilli(*fromTime)
	t := time.UnixMilli(*toTime)

	// Log job size to StdErr
	//fmt.Fprintf(os.Stderr, "Analyzing %d records between\n%s and\n%s\n", count, f.Format(time.RFC822), t.Format(time.RFC822))
	printStdErr("Analyzing %d records between\n  %s and\n  %s\n", COUNT, f.Format(time.RFC822), t.Format(time.RFC822))

	jobsCount := int(math.Ceil(float64(COUNT) / LIMIT))

	//printStdErr("%d %d\n", jobsCount, COUNT)

	jobs := make(chan int, int(jobsCount))

	var wg sync.WaitGroup

	for w := 1; w <= *WORKERS; w++ {
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
	//printStdErr("%d\n", job)
	activities := fetchActivities(*fromTime, *toTime, (job-1)*LIMIT)
	// Display activities
	for i := 0; i < len(activities.Results); i++ {
		a := activities.Results[i]
		split := strings.Split(a.EntityPath, "/")
		split = split[0 : *depth+1]
		entry := fmt.Sprintf("%s\t%s", a.AccessLocation, strings.Join(split, "/"))
		CACHE.Add(entry, 1, 0)
		STATUS[w-1] += 1
		updateJobStatus()
	}
}

func updateJobStatus() {
	done := 0
	//printStdErr("%s\n", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(STATUS)), " "), "[]"))
	for i := 0; i < len(STATUS); i++ {
		done += STATUS[i]
	}
	percent := int(100 * int64(done) / COUNT)
	//printStdErr("Done %d%% %d %d\n", percent, done, COUNT)
	printStdErr("\rDone %d%%", percent)
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

	retries := HTTP_RETRIES
	var resp *http.Response
	for retries > 0 {
		resp, err = client.Do(req)
		if err != nil {
			retries -= 1
		} else {
			break
		}
	}

	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		printStdErr("Authentication failed, please check API KEY and API Endpoint\n")
		os.Exit(2)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var activities Activities

	json.Unmarshal(b, &activities)

	return &activities
}
