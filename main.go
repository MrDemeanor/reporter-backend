package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/tealeg/xlsx"
)

// Test comment
type Test struct {
	Name        string
	Identifier  string
	DatFile     string
	DatFileName string
	LOFile      string
	LOFileName  string
}

type LOKey struct {
	Key string
}

func ProduceIntermediateXLSX(w http.ResponseWriter, r *http.Request) {

	jsn, err := ioutil.ReadAll(r.Body)

	if err != nil {
		log.Fatal("Error while reading r.body: ", err)
	}

	var tests []Test

	err = json.Unmarshal(jsn, &tests)

	if err != nil {
		log.Fatal("Error while performing Unmarshal: ", err)
	}

	students := make(map[string][]string)

	reg, err := regexp.Compile("[^a-zA-Z]+")

	for testNum, test := range tests {
		dataFile := strings.Split(test.DatFile, "\n")

		for _, line := range dataFile {
			studentIdentifier := reg.ReplaceAllString(line, "")

			if studentIdentifier != "" {

				if string(studentIdentifier[0]) == "A" {
					studentIdentifier = strings.TrimLeft(studentIdentifier, "A")
				}

				lineArray := strings.Fields(line)

				_, studentExists := students[studentIdentifier]

				if studentExists {
					students[studentIdentifier][testNum+1] = lineArray[len(lineArray)-2]
				} else {
					newEntry := make([]string, len(tests)+1)
					newEntry[0] = studentIdentifier
					newEntry[testNum+1] = lineArray[len(lineArray)-2]
					students[studentIdentifier] = newEntry
				}
			}
		}
	}

	var finalProduct [][]string

	for _, test := range tests {
		LOKey := []string{test.LOFile}
		finalProduct = append(finalProduct, LOKey)
	}

	keys := make([]string, 0, len(students))

	for key := range students {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		finalProduct = append(finalProduct, students[key])
	}

	jsn, err = json.Marshal(finalProduct)

	if err != nil {
		log.Fatal("Error performing Marshal operation: ", err)
	}

	w.Write(jsn)
}

func getNumTests(sheet *xlsx.Sheet) (numTests int, err error) {

	for _, row := range sheet.Rows {
		if unicode.IsNumber(rune(row.Cells[0].String()[0])) {
			numTests++
		} else {
			return numTests, nil
		}
	}

	return 0, err
}

func setLOKeys(sheet *xlsx.Sheet, numTests int) (LOKeys []string) {
	for i := 0; i < numTests; i++ {
		LOKeys = append(LOKeys, sheet.Rows[i].Cells[0].String())
	}

	return LOKeys
}

func addFirstRow(numLearningObjectives int) (firstRow []string) {

	firstRow = append(firstRow, "")

	for i := 0; i < numLearningObjectives; i++ {
		firstRow = append(firstRow, "Learning Objective "+strconv.Itoa(i+1))
	}

	return firstRow
}

func getNumQuestionsPerLO(LOKeys []string, numQuestionsPerLO map[int]int) {
	for _, key := range LOKeys {
		for _, num := range key {
			numQuestionsPerLO[int(num-'0')]++
		}
	}
}

func getNumLearningObjectives(LOKeys []string) (numLearningObjectives int) {

	for _, key := range LOKeys {
		for _, num := range key {
			if int(num-'0') > numLearningObjectives {
				numLearningObjectives = int(num - '0')
			}
		}
	}

	return numLearningObjectives
}

func createExcelDocument(finalOutput [][]string) (file *xlsx.File) {
	var sheet *xlsx.Sheet
	var row *xlsx.Row
	var cell *xlsx.Cell
	var err error

	file = xlsx.NewFile()
	sheet, err = file.AddSheet("FinalOutput")
	if err != nil {
		fmt.Printf(err.Error())
	}

	for _, student := range finalOutput {
		row = sheet.AddRow()
		for _, studentCell := range student {
			cell = row.AddCell()
			cell.Value = studentCell
		}
	}

	return file

}

func ProduceFinalXLSX(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	file, _, err := r.FormFile("file")

	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}

	defer file.Close()

	// Create a temporary file within our temp-images directory that follows
	// a particular naming pattern
	UUID := uuid.Must(uuid.NewRandom())
	tempFile, err := ioutil.TempFile("uploaded", UUID.String()+"-*.xlsx")

	if err != nil {
		log.Fatal("Could not create temporary file: ", err)
	}
	defer tempFile.Close()

	// read all of the contents of our uploaded file into a
	// byte array
	fileBytes, err := ioutil.ReadAll(file)

	if err != nil {
		log.Fatal("Could not read contents of uploaded file")
	}

	// write this byte array to our temporary file
	tempFile.Write(fileBytes)

	xlFile, err := xlsx.OpenFile(tempFile.Name())

	if err != nil {
		fmt.Println(err)
	}

	if err != nil {
		fmt.Printf(err.Error())
	}

	intermediateSheet := xlFile.Sheets[0]

	var numQuestionsPerLO = make(map[int]int)

	numTests, err := getNumTests(intermediateSheet)

	if err != nil {
		log.Fatal(err)
	}

	// Get LOKeys
	LOKeys := setLOKeys(intermediateSheet, numTests)

	// Get number of learning objectives
	// TODO: TURN INTO GOLANG LIBRARY AND POST ON GITHUB
	numLearningObjectives := getNumLearningObjectives(LOKeys)

	// Add first row
	var finalOutput [][]string
	finalOutput = append(finalOutput, addFirstRow(numLearningObjectives))

	// Get number of questions per learning objective
	getNumQuestionsPerLO(LOKeys, numQuestionsPerLO)

	// Loop through each student
	for i := numTests; i < len(intermediateSheet.Rows); i++ {
		var nextStudent []string
		nextStudent = append(nextStudent, intermediateSheet.Rows[i].Cells[0].String())

		studentMetrics := make(map[int]int)

		// Figure out how many of each kind of learning objective the student answered correctly
		for j := 1; j < len(intermediateSheet.Rows[i].Cells); j++ {
			for index, char := range intermediateSheet.Rows[i].Cells[j].String() {
				if char == '1' {
					studentMetrics[int(LOKeys[j-1][index]-'0')]++
				}
			}
		}

		// Add those percentage values to the current student
		for j := 0; j < numLearningObjectives; j++ {
			percentage := (float64(studentMetrics[j+1]) / float64(numQuestionsPerLO[j+1])) * 100
			nextStudent = append(nextStudent, fmt.Sprintf("%.2f", percentage))
		}

		finalOutput = append(finalOutput, nextStudent)

	}

	jsn, err := json.Marshal(finalOutput)

	if err != nil {
		log.Fatal("Error performing Marshal operation: ", err)
	}

	w.Write(jsn)

	os.Remove(tempFile.Name())

}

func main() {
	router := mux.NewRouter()

	headers := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})
	methods := handlers.AllowedMethods([]string{"GET", "POST"})
	origins := handlers.AllowedOrigins([]string{"*"})

	router.HandleFunc("/api/intermediate_xlsx", ProduceIntermediateXLSX).Methods("POST")
	router.HandleFunc("/api/final_xlsx", ProduceFinalXLSX).Methods("POST")

	http.ListenAndServe(":8080", handlers.CORS(headers, methods, origins)(router))
}
