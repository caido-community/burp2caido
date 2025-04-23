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
	"flag"
	"log"
	"fmt"
)

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

	converter, err := NewConverter(*caido)
	if err != nil {
		log.Fatal(err)
	}
	defer converter.Close()

	if err := converter.ConvertBurpFile(*burpsuite); err != nil {
		log.Fatal(err)
	}

	log.Println("\033[32m[INFO] Updated Caido databases successfully.\033[0m")
}
