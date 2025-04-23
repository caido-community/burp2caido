package main

/*
BurpToCaido v1.2 by monke
---
This is a utility to convert HTTP history from Burpsuite to Caido.
Burpsuite HTTP history is exported in XML format. This is formatted and
inserted into Caido's SQLite databases.

Usage:
- Run the binary, specifying the input file and the location of Caido's projects.
./burptocaido --burpsuite <path to XML file> --caido <path to Caido project folder containing database.caido>
*/

import (
	"database/sql"
	"encoding/base64"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Item struct {
	// An Item is a HTTP request/response pair within Burpsuite's exported XML.
	Time           string `xml:"time"`
	URL            string `xml:"url"`
	Host           string `xml:"host"`
	Port           int    `xml:"port"`
	Protocol       string `xml:"protocol"`
	Method         string `xml:"method"`
	Path           string `xml:"path"`
	Extension      string `xml:"extension"`
	Request        string `xml:"request"`
	Status         int    `xml:"status"`
	ResponseLength int    `xml:"responselength"`
	MimeType       string `xml:"mimetype"`
	Response       string `xml:"response"`
	Comment        string `xml:"comment"`
}

func main() {

	var banner = fmt.Sprintf(`
██████╗ ██╗   ██╗██████╗ ██████╗ ██████╗  ██████╗ █████╗ ██╗██████╗  ██████╗
██╔══██╗██║   ██║██╔══██╗██╔══██╗╚════██╗██╔════╝██╔══██╗██║██╔══██╗██╔═══██╗
██████╔╝██║   ██║██████╔╝██████╔╝ █████╔╝██║     ███████║██║██║  ██║██║   ██║
██╔══██╗██║   ██║██╔══██╗██╔═══╝ ██╔═══╝ ██║     ██╔══██║██║██║  ██║██║   ██║
██████╔╝╚██████╔╝██║  ██║██║     ███████╗╚██████╗██║  ██║██║██████╔╝╚██████╔╝ by monke v%s
╚═════╝  ╚═════╝ ╚═╝  ╚═╝╚═╝     ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝╚═════╝  ╚═════╝
`, "1.2")
	log.Println(banner)

	burpsuite := flag.String("burp", "", "Path to Burpsuite XML file")
	caido := flag.String("caido", "", "Path to Caido project path")
	flag.Parse()

	if *burpsuite == "" {
		log.Fatal("The --burp flag is required.")
	}

	if *caido == "" {
		log.Fatal("The --caido flag is required.")
	}

	log.Println("[INFO] Using Caido path: " + *caido)
	log.Println("[INFO] Using Burpsuite path: " + *burpsuite)

	// Open the Caido database
	db, err := openDB(*caido)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Open the Burpsuite XML file
	xmlFile, err := os.Open(*burpsuite)
	if err != nil {
		log.Fatal("Error opening Burpsuite XML file.")
	}
	defer xmlFile.Close()

	// Parse the XML file and insert the data into the Caido database
	decoder := xml.NewDecoder(xmlFile)
	for {
		token, _ := decoder.Token()
		if token == nil {
			break
		}

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "item" {
				var item Item
				decoder.DecodeElement(&item, &se)
				err := insertData(db, item)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	log.Println("\033[32m[INFO] Updated Caido databases successfully.\033[0m")
}

func openDB(projectPath string) (*sql.DB, error) {
	// Check if the database exists
	dbPath := projectPath + "/database.caido"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Caido main database does not exist")
	}

	// Open the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("Error opening database.caido: %v", err)
	}

	// Attach the raw database
	dbRawPath := projectPath + "/database_raw.caido"
	_, err = db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS raw", dbRawPath))
	if err != nil {
		return nil, fmt.Errorf("Error attaching database_raw.caido: %v", err)
	}

	return db, nil
}

func insertData(db *sql.DB, item Item) error {
	responseID, err := insertResponse(db, item)
	if err != nil {
		return err
	}
	requestID, err := insertRequest(db, responseID, item)
	if err != nil {
		return err
	}
	_, err = insertIntercept(db, requestID)
	if err != nil {
		return err
	}
	return nil
}

func insertResponse(db *sql.DB, item Item) (int64, error) {
	responseData, _ := base64.StdEncoding.DecodeString(item.Response)
	timestamp := getTimestamp(item)

	var rawResponseID int64
	err := db.QueryRow("INSERT INTO raw.responses_raw (data, source, alteration) VALUES (?, 'intercept', 'none') RETURNING id", responseData).Scan(&rawResponseID)
	if err != nil {
		return 0, err
	}

	var responseID int64
	err = db.QueryRow("INSERT INTO responses (status_code, raw_id, length, alteration, edited, roundtrip_time, created_at) VALUES (?, ?, ?, 'none', 0, 0, ?) RETURNING id",
		item.Status, rawResponseID, item.ResponseLength, timestamp).Scan(&responseID)
	if err != nil {
		return 0, err
	}

	return responseID, nil
}

func insertRequest(db *sql.DB, responseID int64, item Item) (int64, error) {
	requestData, _ := base64.StdEncoding.DecodeString(item.Request)
	timestamp := getTimestamp(item)

	var rawRequestID int64
	err := db.QueryRow("INSERT INTO raw.requests_raw (data, source, alteration) VALUES (?, 'intercept', 'none') RETURNING id", requestData).Scan(&rawRequestID)
	if err != nil {
		return 0, err
	}

	var metadataID int64
	err = db.QueryRow("INSERT INTO requests_metadata DEFAULT VALUES RETURNING id").Scan(&metadataID)
	if err != nil {
		return 0, err
	}

	var requestID int64
	err = db.QueryRow("INSERT INTO requests (host, method, path, length, port, is_tls, raw_id, query, response_id, source, created_at, metadata_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'intercept', ?, ?) RETURNING id",
		item.Host, item.Method, item.Path, len(requestData), item.Port, item.Protocol == "https", rawRequestID, "", responseID, timestamp, metadataID).Scan(&requestID)
	if err != nil {
		return 0, err
	}

	return requestID, nil
}

func insertIntercept(db *sql.DB, requestId int64) (int64, error) {
	var interceptID int64
	err := db.QueryRow("INSERT INTO intercept_entries (request_id) VALUES (?) RETURNING id", requestId).Scan(&interceptID)
	if err != nil {
		return 0, err
	}

	return interceptID, nil
}

func getTimestamp(item Item) int64 {
	layout := "Mon Jan 02 15:04:05 MST 2006"
	parsedTime, err := time.Parse(layout, item.Time)
	if err != nil {
		log.Fatalf("Failed to parse datetime: %v", err)
	}

	timestamp := parsedTime.UnixNano() / int64(time.Millisecond)

	return timestamp
}
