package utils

import (
	"encoding/csv"
	"log"
	"os"
)

// WriteCsv wraps around writter facilities in csv module
func WriteCsv(records [][]string) {
	w := csv.NewWriter(os.Stdout)

	for _, record := range records {
		if err := w.Write(record); err != nil {
			log.Fatalln("error writing the record", err)
		}
	}

	w.Flush()
}
