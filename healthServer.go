package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"gopkg.in/gomail.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Sensor struct {
	Datetime string    `json:"Datetime" binding:"required"`
	Data     []float32 `json:"Data" binding:"required"`
}

type Data struct {
	BeaconAddress string   `json:"Beacon Address" binding:"required"`
	SessionNumber int      `json:"Session Number" binding:"required"`
	Datetime      string   `json:"Datetime" binding:"required"`
	TimeStart     int32    `json:"TimeStart" binding:"required"`
	TimeEnd       int32    `json:"TimeEnd" binding:"required"`
	MaxTemp       float32  `json:"MaxTemp" binding:"required"`
	MinTemp       float32  `json:"MinTemp" binding:"required"`
	AvgTemp       float32  `json:"AvgTemp" binding:"required"`
	AvgHumidity   float32  `json:"AvgHumidity" binding:"required"`
	SensorLog     []Sensor `json:"SensorLog" binding:"required"`
	SurveyResults []int    `json:"SurveyResults" binding:"required"`
}

var questions []string

func main() {
	configf := flag.String("config-file", "app.cfg", "The file to use for config")
	ac := LoadAppConfig(*configf)
	ac.sendTestEmail()
	reportChannel := make(chan *Data, 32)
	var test Data
	test.BeaconAddress = ":D"
	reportChannel <- &test

	r := initRouter(reportChannel)
	// Run in the background
	go reportLoop(ac, reportChannel)

	loadQuestions()
	r.Run(":" + ac.Port)
}

func initRouter(reportChannel chan<- *Data) *gin.Engine {

	r := gin.Default()
	r.Use(static.Serve("/", static.LocalFile("files", true)))
	r.Use(cors.Default())

	r.GET("/results", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	r.POST("/results/:expName", func(c *gin.Context) {
		experimentName := c.Param("expName")

		//Read JSON data from request body
		x, _ := ioutil.ReadAll(c.Request.Body)
		var d Data
		err := json.Unmarshal(x, &d)

		da := createDateString(d)

		if err != nil {
			// Error parsing JSON data
			log.Println(err)
			c.JSON(http.StatusBadRequest, gin.H{"Success": false, "Error": err})
			return
		}

		// Check if file already exists
		version := 0
		dir, _ := os.Getwd()
		_, err = os.Stat(formatFileName(da, dir, experimentName, d.SessionNumber, createVersionString(version)+"_sensor.csv"))

		for err == nil {
			version++
			_, err = os.Stat(formatFileName(da, dir, experimentName, d.SessionNumber, createVersionString(version)+"_sensor.csv"))
		}

		//Write the data to csv
		if checkData(d) {
			// Send data to be reported on
			reportChannel <- &d

			writeCSV(CSV_SENSOR, d, formatAddress(experimentName), createVersionString(version))
			writeCSV(CSV_CONFIG, d, formatAddress(experimentName), createVersionString(version))
			writeCSV(CSV_SURVEY, d, formatAddress(experimentName), createVersionString(version))
			c.JSON(http.StatusCreated, gin.H{"Success": true})
		} else {
			// TODO(brad) this says true, isn't it meant to be false?
			c.JSON(http.StatusPreconditionFailed, gin.H{"Success": true, "Error": "Empty values"})
		}

	})
	return r
}

func createDateString(d Data) string {
	t := time.Now()
	y := t.Format(time.RFC3339)
	s := strings.Split(y, "T")
	date := s[0]
	return date
}

func createVersionString(version int) string {
	if version <= 0 {
		return ""
	} else {
		return "(" + strconv.Itoa(version) + ")"
	}
}

func formatFileName(date string, dir string, experimentName string, SessionNumber int, csvname string) string {
	var folder string
	if runtime.GOOS == "windows" {
		folder = "\\data\\"
	} else {
		folder = "/data/"
	}
	return dir + folder + "_" + experimentName + "_" + date + "_" + strconv.Itoa(SessionNumber) + csvname
}

func formatAddress(address string) string {
	return strings.Replace(address, ":", "", -1)
}

//Panics if error occurred
func check(err error) {
	if err != nil {
		panic(err)
	}
}

//Checks JSON data structure to ensure no fields are empty
func checkData(d Data) bool {
	if d.Datetime == "" ||
		d.BeaconAddress == "" ||
		d.SensorLog == nil {
		return false
	}

	for _, point := range d.SensorLog {
		if point.Datetime == "" ||
			len(point.Data) != 5 {
			return false
		}
	}
	return true
}

const (
	CSV_SENSOR = iota
	CSV_CONFIG
	CSV_SURVEY
)

//Outputs data to csv file, for "position", "accelerometer", and "config" data
func writeCSV(csvType int, d Data, experimentName string, version string) bool {
	t := time.Now()
	dir, _ := os.Getwd()

	var csvname string
	switch csvType {
	case CSV_SENSOR:
		csvname = version + "_sensor.csv"
	case CSV_CONFIG:
		csvname = version + "_config.csv"
	case CSV_SURVEY:
		csvname = version + "_survey.csv"
	}

	da := createDateString(d)
	num := 1

	test := formatFileName(da, dir, experimentName, 1, csvname)
	if _, err := os.Stat(test); err == nil {
		//1 exists
		num = 2
	}
	test1 := formatFileName(da, dir, experimentName, 2, csvname)
	if _, err := os.Stat(test1); err == nil {
		//2 exists
		num = 3
	}

	//file, fErr := os.Create(formatFileName(da, dir, experimentName, d.SessionNumber,csvname))
	file, fErr := os.Create(formatFileName(da, dir, experimentName, num, csvname))
	check(fErr)

	writer := bufio.NewWriter(file)
	if csvType == CSV_SENSOR {
		fmt.Fprintf(writer, "\"datetime\",\"audio\",\"pressure\",\"temp\",\"humidity\",\"light\"")
		for _, data := range d.SensorLog {
			fmt.Fprintf(writer, "\n\"%s\",%f,%f, %f,%f,%f", data.Datetime, data.Data[0], data.Data[1], data.Data[2], data.Data[3], data.Data[4])
		}
	} else if csvType == CSV_CONFIG {
		fmt.Fprintf(writer, "{\n")
		fmt.Fprintf(writer, "\t\"Beacon Address\": \"%s\"\n", d.BeaconAddress)
		fmt.Fprintf(writer, "\t\"Session Number\": \"%d\"\n", d.SessionNumber)
		fmt.Fprintf(writer, "\t\"Datetime\": \"%s\"\n", d.Datetime)
		fmt.Fprintf(writer, "\t\"Received Datetime\": \"%s\"\n", t.Format(time.RFC3339))
		fmt.Fprintf(writer, "\t\"Time Start\": \"%d\"\n", d.TimeStart)
		fmt.Fprintf(writer, "\t\"Time End\": \"%d\"\n", d.TimeEnd)
		fmt.Fprintf(writer, "\t\"Max Temp\": \"%f\"\n", d.MaxTemp)
		fmt.Fprintf(writer, "\t\"Min Temp\": \"%f\"\n", d.MinTemp)
		fmt.Fprintf(writer, "\t\"Avg Temp\": \"%f\"\n", d.AvgTemp)
		fmt.Fprintf(writer, "\t\"Avg Humidity\": \"%f\"\n", d.AvgHumidity)
		fmt.Fprintf(writer, "}")
	} else if csvType == CSV_SURVEY {
		for i := 0; i < 3; i++ {
			fmt.Fprintf(writer, "%s: %d\n", questions[i], d.SurveyResults[i])
		}
		for i := 3; i < 12; i++ {
			fmt.Fprintf(writer, "%s: %s\n", questions[i], toYesNo(d.SurveyResults[i]))
		}
		fmt.Fprintf(writer, "%s: %d\n", questions[12], d.SurveyResults[12])
		for i := 13; i < 36; i++ {
			fmt.Fprintf(writer, "%s: %s\n", questions[i], toYesNo(d.SurveyResults[i]))
		}
		fmt.Fprintf(writer, "%s: %d\n", questions[36], d.SurveyResults[36])
	}
	writer.Flush()
	file.Close()
	return true
}

func toYesNo(selection int) string {
	if selection == 1 {
		return "YES"
	}
	return "NO"
}

func loadQuestions() {
	dir, _ := os.Getwd()
	file, err := os.Open(dir + "/src/questions.txt")
	check(err)
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		questions = append(questions, scanner.Text())
	}

	err = scanner.Err()
	check(err)
}

func LoadAppConfig(file string) *AppConfig {
	var ac AppConfig
	f, err := os.Open(file)
	if err != nil {
		log.Fatalf("Failed to open config, does the config file (%s) exist? Error: %s", file, err)
	}
	dec := json.NewDecoder(f)
	if err = dec.Decode(&ac); err != nil {
		log.Fatalf("Failed to decode json. Error: %s", err)
	}
	return &ac
}

type AppConfig struct {
	// Port is a string so we don't need to a string conversion
	Port           string
	ReportHours    []int
	Sender         string
	To             []string
	SMTPHost       string
	SMTPPort       int
	SMTPUser       string
	SMTPPassphrase string
}

func inArray(v int, a []int) bool {
	for _, check := range a {
		if v == check {
			return true
		}
	}
	return false
}

// Accepts data
func reportLoop(ac *AppConfig, dc <-chan *Data) {
	var counters map[string]int
	// Day-Hour for preventing double sends
	var lastsent string = "-1--1"
	// Last time we sent for duration
	var lasttime time.Time

	counters = make(map[string]int)
	timer := time.Tick(time.Millisecond * 500)
	for {
		select {
		case d, ok := <-dc:
			if !ok {
				// Closed channel
				return
			}
			a := d.BeaconAddress
			n, ok := counters[a]
			if !ok {
				// Safety set n = 0
				n = 0
			}
			counters[a] = n + 1
		case <-timer:
			now := time.Now()
			// We only care about hours that are requested
			if !inArray(now.Hour(), ac.ReportHours) {
				continue
			}
			// Make sure we don't send the same email twice
			lastsentfmt := fmt.Sprintf("%d-%d", now.Day(), now.Hour())
			if lastsentfmt == lastsent {
				continue
			}
			lastsent = lastsentfmt
			// Send report
			d := compileReport(counters, now, lasttime)
			if err := ac.sendEmail(d); err != nil {
				log.Printf("Failed to send email with %s", err)
			}
			lasttime = now
			// Clear counters
			counters = make(map[string]int)
		}
	}
}

func compileReport(counters map[string]int,
	now, lasttime time.Time) *bytes.Buffer {

	buff := new(bytes.Buffer)
	fmt.Fprintf(buff, "Hey,<br><br>Between %s and %s the following submission were made:<br><br>",
		lasttime.Format(time.RFC3339), now.Format(time.RFC3339))
	fmt.Fprintln(buff, `<table>
    <thead>
        <tr>
            <th>ID</th><th>Count</th>
        </tr>
    </thead>
    <tbody>`)
	for k, v := range counters {
		fmt.Fprintf(buff, "        <tr><td>%s</td><td>%d</td></tr>\n", k, v)
	}
	fmt.Fprint(buff, "    </tbody>\n</table>")

	return buff
}

// sendEmail sends an email given the app config with HTML data in buffer d
func (ec *AppConfig) sendTestEmail() {
	m := gomail.NewMessage()
	m.SetHeader("From", ec.Sender)
	m.SetHeader("To", ec.To...)
	m.SetHeader("Subject", "Experiment Server Started: "+time.Now().Format(time.RFC3339))
	m.SetBody("text/html", `Server started`)
	dialer := gomail.NewDialer(ec.SMTPHost, ec.SMTPPort, ec.SMTPUser, ec.SMTPPassphrase)
	if err := dialer.DialAndSend(m); err != nil {
		log.Fatalf("Fatal error occured while trying to send the test email: %s", err)
	}
}

// sendEmail sends an email given the app config with HTML data in buffer d
func (ec *AppConfig) sendEmail(d *bytes.Buffer) error {
	m := gomail.NewMessage()
	m.SetHeader("From", ec.Sender)
	m.SetHeader("To", ec.To...)
	m.SetHeader("Subject", "Experiment Report: "+time.Now().Format(time.RFC3339))
	m.SetBody("text/html", d.String())
	dialer := gomail.NewDialer(ec.SMTPHost, ec.SMTPPort, ec.SMTPUser, ec.SMTPPassphrase)
	return dialer.DialAndSend(m)
}
