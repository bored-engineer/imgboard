package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	httpPort = flag.Uint("http-port", 8080, "http port to run on")
)

var (
	boundry = "spiderman"
	m       = image.NewRGBA(image.Rect(0, 0, 800, 800))
	mut     = sync.RWMutex{}

	numOnline int64

	bc = newBroadcast()
)

func init() {
	flag.Parse()
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<form method="get" action="/click">
	<table>
	<tr><td>
	<h5>Color</h5>
	<input type="color" name="color" value="#e66465">
	
	<br />
	
	<h5>Size</h5>
	<input type="range" name="size" min="1" max="99" value="10">
	</td><td><input type="image" name="imgbtn" src="/image"></td></table>
</form>`)
}

func mjpegHandler(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&numOnline, 1)

	n, ok := w.(http.CloseNotifier)
	if !ok {
		http.Error(w, "cannot stream - no closer", http.StatusInternalServerError)
		return
	}

	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "cannot stream - no flush", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-type", "multipart/x-mixed-replace; boundary="+boundry)

	datac := bc.Register()

	data := imgbytes()
	for i := 0; i <= 10; i++ {
		writeFrame(w, data)
		f.Flush()
	}
	fmt.Println("...")

	for {

		t := time.NewTimer(5 * time.Second)

		select {
		case data = <-datac:
			for i := 0; i <= 1; i++ {
				writeFrame(w, data)
				f.Flush()
			}
		case <-n.CloseNotify():
			atomic.AddInt64(&numOnline, -1)
			fmt.Println("...closed")

			bc.Clear(datac)

			return
		case <-t.C:
			writeFrame(w, data)
			f.Flush()
		}

	}
}

func writeFrame(w http.ResponseWriter, data []byte) {
	fmt.Fprintf(w, "--%s\n", boundry)
	fmt.Fprint(w, "Content-Type: image/jpeg\n\n")
	w.Write(data)
	fmt.Fprint(w, "\n\n")
}

func clickHandler(w http.ResponseWriter, r *http.Request) {
	x, err := strconv.Atoi(r.FormValue("imgbtn.x"))
	if err != nil {
		http.Error(w, "invalid int for x", 400)
		return
	}

	y, err := strconv.Atoi(r.FormValue("imgbtn.y"))
	if err != nil {
		http.Error(w, "invalid int for y", 400)
		return
	}

	c := color.RGBA{255, 255, 0, 255}

	s, err := strconv.Atoi(r.FormValue("size"))
	if err != nil {
		http.Error(w, "invalid int for size", 400)
		return
	}

	if s < 0 || s > 100 {
		http.Error(w, "invalid size", 400)
		return
	}

	fc := r.FormValue("color")
	if fc != "" {
		if len(fc) != 7 || fc[0] != '#' {
			http.Error(w, "invalid color", 400)
			return
		}

		cr, err1 := strconv.ParseUint(fc[1:3], 16, 8)
		cg, err2 := strconv.ParseUint(fc[3:5], 16, 8)
		cb, err3 := strconv.ParseUint(fc[5:7], 16, 8)

		if err1 != nil || err2 != nil || err3 != nil {
			http.Error(w, "invalid color", 400)
			return
		}

		c = color.RGBA{uint8(cr), uint8(cg), uint8(cb), 255}
	}

	mut.Lock()
	for xx := 0 - s; xx <= s; xx++ {
		for yy := 0 - s; yy <= s; yy++ {
			m.Set(x+xx, y+yy, c)
		}
	}
	mut.Unlock()

	bc.Broadcast(imgbytes())

	fmt.Println(x, y)
	w.WriteHeader(http.StatusNoContent)

}

func imgbytes() []byte {
	mut.Lock()
	defer mut.Unlock()

	var bb bytes.Buffer
	jpeg.Encode(&bb, m, &jpeg.Options{Quality: 70})
	return bb.Bytes()
}

func main() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/click", clickHandler)
	http.HandleFunc("/image", mjpegHandler)

	go func() {
		for {
			log.Println(atomic.LoadInt64(&numOnline), " users online")
			time.Sleep(10 * time.Second)
		}
	}()

	err := http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), nil)
	if err != nil {
		panic(err)
	}
}
