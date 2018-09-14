package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
	"strings"
	"runtime"
)

type Sensor struct {
	Datetime string    `json:"Datetime" binding:"required"`
	Data     []float32 `json:"Data" binding:"required"`
}

type Data struct {
	BeaconAddress string `json:"Beacon Address" binding:"required"`
	SessionNumber int   `json:"Session Number" binding:"required"`
	Datetime    string   `json:"Datetime" binding:"required"`
	TimeStart   int32    `json:"TimeStart" binding:"required"`
	TimeEnd     int32    `json:"TimeEnd" binding:"required"`
	MaxTemp     float32  `json:"MaxTemp" binding:"required"`
	MinTemp     float32  `json:"MinTemp" binding:"required"`
	AvgTemp     float32  `json:"AvgTemp" binding:"required"`
	AvgHumidity float32  `json:"AvgHumidity" binding:"required"`
	SensorLog   []Sensor `json:"SensorLog" binding:"required"`
	SurveyResults   []int `json:"SurveyResults" binding:"required"`
}

var questions []string

func main() {
	r := initRouter()
	loadQuestions()
	r.Run(":5000")
}

func initRouter() *gin.Engine {

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

		da:=createDateString(d)

		//Check if JSON data is valid
		if err == nil {
			
			//Check if file already exists
			version := 0
			dir, _ := os.Getwd()
			_, err := os.Stat(formatFileName(da, dir, experimentName, d.SessionNumber, createVersionString(version) + "_sensor.csv"))

			for err == nil{
				version++
				_, err = os.Stat(formatFileName(da, dir, experimentName, d.SessionNumber, createVersionString(version) + "_sensor.csv"))
			}

			//Write the data to csv
			if checkData(d) {
				writeCSV(CSV_SENSOR, d, formatAddress(experimentName), createVersionString(version))
				writeCSV(CSV_CONFIG, d, formatAddress(experimentName), createVersionString(version))
				writeCSV(CSV_SURVEY, d, formatAddress(experimentName), createVersionString(version))
				c.JSON(http.StatusCreated, gin.H{"Success": true})
			} else {
				c.JSON(http.StatusPreconditionFailed, gin.H{"Success": true, "Error": "Empty values"})
			}

		} else {
			//Error parsing JSON data
			fmt.Println(err)
			c.JSON(http.StatusBadRequest, gin.H{"Success": false, "Error": err})
		}
	})
	return r
}

func createDateString(d Data) string{
	t:=time.Now()
	y:=t.Format(time.RFC3339)
	s:= strings.Split(y, "T")
	date := s[0]
	return date
}

func createVersionString(version int) string{
	if version <= 0{
		return ""
	}else{
		return "(" + strconv.Itoa(version) + ")"
	}
}

func formatFileName(date string, dir string, experimentName string, SessionNumber int, csvname string) string{
	var folder string
	if runtime.GOOS == "windows"{
		folder = "\\data\\"
	} else {
		folder = "/data/"
	}
	return dir + folder + "_"+experimentName + "_" + date + "_" + strconv.Itoa(SessionNumber) + csvname
}

func formatAddress(address string) string{
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

	da:=createDateString(d)
	num:=1

	test:= formatFileName(da, dir, experimentName, 1,csvname)
	if _, err := os.Stat(test); err == nil {
		//1 exists
	  num=2;
	}
	test1:= formatFileName(da, dir, experimentName, 2,csvname)
	if _, err := os.Stat(test1); err == nil {
		//2 exists
	  num=3;
	}
	

	//file, fErr := os.Create(formatFileName(da, dir, experimentName, d.SessionNumber,csvname))
	file, fErr := os.Create(formatFileName(da, dir, experimentName, num ,csvname))
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

func toYesNo(selection int) string{
	if selection == 1 {
		return "YES"
	}
	return "NO"
}



func loadQuestions(){
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