// Package filedb is a simple database based on files.
package filedb

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nxadm/tail"
)

type Filedb struct {
	File     *os.File
	FilePath string

	ToMySQLHandler func([]string) error
}

func New(filePath string) (fdb *Filedb, err error) {
	fdb = &Filedb{
		FilePath: filePath,
	}
	err = fdb.Open()

	return
}

func (f *Filedb) Open() (err error) {
	err = os.MkdirAll(filepath.Dir(f.FilePath), 0755)
	if err != nil {
		return
	}

	f.File, err = os.OpenFile(f.FilePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	return
}

func (f *Filedb) Close() (err error) {
	if f.File == nil {
		return
	}

	err = f.File.Close()
	if err != nil {
		return
	}

	f.File = nil

	return
}

// const lineSeparator = "\x1E" // Define special separator

func (f *Filedb) WriteLine(s string) (err error) {
	_, err = f.File.WriteString(s)
	if err != nil {
		log.Println("WriteLine err:", err)
		return
	}

	return
}

// ReadLastLine reads the last non-empty line of the file
func (f *Filedb) ReadLastLine() (s string, err error) {
	stat, err := f.File.Stat()
	if err != nil {
		return
	}

	// Since we don't know how many bytes the last line has, try to read the last 1024 bytes, and extract the last line based on \n
	var b []byte
	var off int64
	size := stat.Size()
	if size < 1024 {
		b = make([]byte, size)
	} else {
		b = make([]byte, 1024)
		off = size - 1024
	}

	_, err = f.File.ReadAt(b, off)
	if err != nil {
		return
	}

	txt := strings.Trim(string(b), " \n")
	txts := strings.Split(txt, "\n")

	if len(txts) == 0 {
		return
	}

	s = txts[len(txts)-1]

	return
}

// ReadFirstLine reads the first non-empty line of the file
func (f *Filedb) ReadFirstLine() (s string, err error) {
	_, err = f.File.Seek(0, 0)
	if err != nil {
		return
	}

	reader := bufio.NewReader(f.File)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		line = strings.TrimSpace(line)
		if line != "" {
			s = line
			return s, nil
		}
	}

	return "", io.EOF
}

// Tailf continuously monitors new data writes and passes them to the handler via chan
func (f *Filedb) Tailf(ch chan<- string) (err error) {
	var loc *tail.SeekInfo
	// TODO locate based on SavedLogID or other parameters to avoid starting from the beginning
	// loc = &tail.SeekInfo{Offset: 0, Whence: 0}
	ta, err := tail.TailFile(f.FilePath, tail.Config{
		Follow:        true,
		ReOpen:        true,
		Location:      loc,
		CompleteLines: true,
	})
	if err != nil {
		return
	}

	// var buffer string
	// for line := range ta.Lines {
	// 	if line.Err != nil {
	// 		err = line.Err
	// 		return
	// 	}

	// 	buffer += line.Text
	// 	if strings.HasSuffix(buffer, lineSeparator) {
	// 		ch <- strings.TrimSuffix(buffer, lineSeparator)
	// 		buffer = ""
	// 	} else {
	// 		log.Println("===== buffer:", buffer)
	// 	}
	// }

	for line := range ta.Lines {
		if line.Err != nil {
			// If an error occurs in a line of data, exit and return the error. Do not skip this line directly, as this may cause data disorder.
			err = line.Err
			return
		}

		ch <- line.Text
	}

	return
}

type PerformanceData struct {
	Name      string
	FirstTime time.Time
	LastTime  time.Time
	Size      int
}

func showPerformance(data *PerformanceData, ch chan struct{}) {
	go func() {
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					return
				}
			case <-time.After(30 * time.Second):
			}

			// if !data.LastTime.IsZero() && time.Since(data.LastTime) > 30*time.Second {
			rate := int64(0)
			if data.LastTime.Sub(data.FirstTime).Seconds() > 0 {
				rate = int64(float64(data.Size) / data.LastTime.Sub(data.FirstTime).Seconds())
			}

			resultText := fmt.Sprintf(
				"Benchmark: %s send %d logs to mysql in %s at %s with rate %d/sec\n",
				data.Name, data.Size, data.LastTime.Sub(data.FirstTime),
				data.LastTime.Format(time.RFC3339), rate,
			)

			fmt.Printf("===== %+v", resultText)
			// return
			// }
		}
	}()
}

// ToMySQL parses the logs and writes them to mysql. This is just the control logic, the actual writing is done in f.ToMySQLHandler
func (f *Filedb) ToMySQL(ch <-chan string) (err error) {
	fmt.Printf("===== ToMySQL start with %s\n", f.FilePath)
	defer func() {
		if err != nil {
			fmt.Printf("===== ToMySQL err with %s and err:%v\n", f.FilePath, err)
		} else {
			fmt.Printf("===== ToMySQL end with %s\n", f.FilePath)
		}
	}()

	// ----- Whether to show performance statistics

	perfData := &PerformanceData{
		Name: f.FilePath,
	}
	perfCh := make(chan struct{})
	defer close(perfCh)
	go showPerformance(perfData, perfCh)

	// ----- Read data from ch and write to mysql

	ss := make([]string, 100)

	for {
		size := 1
		if len(ch) > 1 {
			if len(ch) < len(ss) {
				size = len(ch)
			} else {
				size = len(ss)
			}
		}

		var ok bool
		for i := 0; i < size; i++ {
			ss[i], ok = <-ch
			if !ok {
				return
			}
		}

		if perfData.FirstTime.IsZero() {
			perfData.FirstTime = time.Now()
		}

		err = f.ToMySQLHandler(ss[:size])
		if err != nil {
			return
		}

		perfData.LastTime = time.Now()
		perfData.Size += size
	}
}
