package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Item struct {
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

type Converter struct {
	db *sql.DB
}

func NewConverter(projectPath string) (*Converter, error) {
	db, err := openDB(projectPath)
	if err != nil {
		return nil, err
	}
	return &Converter{db: db}, nil
}

func (c *Converter) Close() error {
	return c.db.Close()
}

func (c *Converter) ConvertBurpFile(path string) error {
	xmlFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Error opening Burpsuite XML file: %v", err)
	}
	defer xmlFile.Close()

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
				if err := decoder.DecodeElement(&item, &se); err != nil {
					return fmt.Errorf("error decoding element: %v", err)
				}
				if err := c.insertData(item); err != nil {
					return fmt.Errorf("error inserting data: %v", err)
				}
			}
		}
	}
	return nil
}

func (c *Converter) insertData(item Item) error {
	responseID, err := c.insertResponse(item)
	if err != nil {
		return err
	}
	requestID, err := c.insertRequest(responseID, item)
	if err != nil {
		return err
	}
	_, err = c.insertIntercept(requestID)
	if err != nil {
		return err
	}
	return nil
}

func (c *Converter) insertResponse(item Item) (int64, error) {
	responseData, _ := base64.StdEncoding.DecodeString(item.Response)
	timestamp := getTimestamp(item)

	var rawResponseID int64
	err := c.db.QueryRow("INSERT INTO raw.responses_raw (data, source, alteration) VALUES (?, 'intercept', 'none') RETURNING id", responseData).Scan(&rawResponseID)
	if err != nil {
		return 0, err
	}

	var responseID int64
	err = c.db.QueryRow("INSERT INTO responses (status_code, raw_id, length, alteration, edited, roundtrip_time, created_at) VALUES (?, ?, ?, 'none', 0, 0, ?) RETURNING id",
		item.Status, rawResponseID, item.ResponseLength, timestamp).Scan(&responseID)
	if err != nil {
		return 0, err
	}

	return responseID, nil
}

func (c *Converter) insertRequest(responseID int64, item Item) (int64, error) {
	requestData, _ := base64.StdEncoding.DecodeString(item.Request)
	timestamp := getTimestamp(item)

	var rawRequestID int64
	err := c.db.QueryRow("INSERT INTO raw.requests_raw (data, source, alteration) VALUES (?, 'intercept', 'none') RETURNING id", requestData).Scan(&rawRequestID)
	if err != nil {
		return 0, err
	}

	var metadataID int64
	err = c.db.QueryRow("INSERT INTO requests_metadata DEFAULT VALUES RETURNING id").Scan(&metadataID)
	if err != nil {
		return 0, err
	}

	var requestID int64
	err = c.db.QueryRow("INSERT INTO requests (host, method, path, length, port, is_tls, raw_id, query, response_id, source, created_at, metadata_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'intercept', ?, ?) RETURNING id",
		item.Host, item.Method, item.Path, len(requestData), item.Port, item.Protocol == "https", rawRequestID, "", responseID, timestamp, metadataID).Scan(&requestID)
	if err != nil {
		return 0, err
	}

	return requestID, nil
}

func (c *Converter) insertIntercept(requestId int64) (int64, error) {
	var interceptID int64
	err := c.db.QueryRow("INSERT INTO intercept_entries (request_id) VALUES (?) RETURNING id", requestId).Scan(&interceptID)
	if err != nil {
		return 0, err
	}

	return interceptID, nil
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

	log.Println("[INFO] Opened database.caido")

	// Attach the raw database
	dbRawPath := projectPath + "/database_raw.caido"
	_, err = db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS raw", dbRawPath))
	if err != nil {
		return nil, fmt.Errorf("Error attaching database_raw.caido: %v", err)
	}

	log.Println("[INFO] Attached database_raw.caido")

	return db, nil
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
