package main

import (
	"runtime"
	"image"
	"image/color"
	"image/png"
	"os"
	"math/cmplx"
	"fmt"
	"time"
	"strconv"
)

//convert from HSV to RGB color space
func HSVToRGB(h, s, v float64) color.RGBA {
	hh := h / 60
	i  := int(hh)
	ff := hh - float64(i)

	p := v * (1 - s)
	q := v * (1 - s*ff)
	t := v * (1 - s*(1 - ff))

	var r, g, b float64
	switch i {
	case 0:
		r = v
		g = t
		b = p
	case 1:
		r = q
		g = v
		b = p
	case 2:
		r = p
		g = v
		b = t
	case 3:
		r = p
		g = q
		b = v
	case 4:
		r = t
		g = p
		b = v
	case 5:
		r = v
		g = p
		b = q
	}
	return color.RGBA{uint8(255*r), uint8(255*g), uint8(255*b), 255}
}

//Generates a colour palette 
func GenerateColorPalette (levels int) color.Palette {
	palette := make([]color.Color, levels)
	for i := 0; i < levels; i++{
		n := float64(i) / float64(levels)
		palette[i] = HSVToRGB(360*n, 0.8, 1.0)
	}
	return palette
}

//Split an image into a number of virtal strips
func SplitImage(img *image.Paletted, n int) []image.Image {
	strips := make([]image.Image, n)
	bounds := img.Bounds()
	h_step := int(float64(bounds.Dx()) / float64(n))

	//image width divided by n will not always be an integer
	//so we may have to add/remove a few columns from the last
	//strip 
	offset := bounds.Dx() - n*h_step
	
	for i := 0; i < n; i++{
		x0 := bounds.Min.X + i*h_step
		x1 := bounds.Min.X + (i+1)*h_step
		if i == n - 1 { // if last strip
			x1 += offset
		}
		strips[i] = img.SubImage(image.Rect(x0, bounds.Min.Y, x1, bounds.Max.Y))
	}
	return strips
}

//function for measuring progress
func ProgressBar(max int) func(int) {
	last_percent_str := ""
	return func(val int) {
		percent_float := 100*float64(val) / float64(max)
		percent_str :=  strconv.FormatFloat(percent_float, 'f', 1,  64)
		if(percent_str != last_percent_str){
			fmt.Print("\r", percent_str, "%")
		}
		last_percent_str = percent_str
	}
}

type Pt_f64 struct {
	X, Y float64
}

type Fractal interface {
	At(X, Y int) uint8
}

//Render the fractal to an Paletted, usually a sub-image
//call as a goroutine
func Render(img *image.Paletted, fractal Fractal, channel chan int) {
	num_of_pix := 0
	for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
		for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
			img.SetColorIndex(x, y, fractal.At(x,y))
			num_of_pix += 1
			channel <- num_of_pix
		}
	}
	close(channel)
}

type Mandelbrot struct {
	Start, End Pt_f64
	ImageSize image.Rectangle
	Bailout float64
	MaxIterations int
}

//gets the colour index at pixel X, Y for the Mandelbrot set
//using the escape time algorithm
func (m Mandelbrot) At(X, Y int) uint8 {
	//convert X & Y out of pixel space
	x0 := float64(X - m.ImageSize.Min.X) / float64(m.ImageSize.Max.X - m.ImageSize.Min.X)
	x0 = x0*(m.End.X - m.Start.X) + m.Start.X
	y0 := float64(Y - m.ImageSize.Min.Y) / float64(m.ImageSize.Max.Y - m.ImageSize.Min.Y)
	y0 = y0*(m.End.Y - m.Start.Y) + m.Start.Y

	z := complex(0, 0)
	c := complex(x0, y0)
	
	itr := 0
	for; cmplx.Abs(z) < m.Bailout && itr < m.MaxIterations; itr++ {
		z = z*z + c
	}
	return uint8(itr)
}

func main() {
	//get the number of CPUs, and set the go runtime to utilise them
	cpus := runtime.NumCPU()
	runtime.GOMAXPROCS(cpus)
	
	palette := GenerateColorPalette(255)
	size := 8096.0
	img := image.NewPaletted(image.Rect(0, 0, int(size), int(size*0.4459)), palette)
	fractal := Mandelbrot{
		Start:         Pt_f64{0.276185, 0.479000198},
		End:           Pt_f64{0.367588933, 0.519762846},
		Bailout:       2.0,
		MaxIterations: len(palette),
		ImageSize:     img.Bounds(),
	}

	//for each CPU create a channel and a strip of the image to render
	sub_images := SplitImage(img, cpus)
	channels := make([]chan int, cpus)

	for i := range channels {
		channels[i] = make(chan int)
		go Render(sub_images[i].(*image.Paletted), fractal, channels[i])
	}

	//measure the goroutines progress and wait for them to finish
	progress_bar := ProgressBar(len(img.Pix))
	t := time.Now()
	done := false
	for !done {
		pixels_processed := 0
		done = true
		for i := range channels {
			val, open := <-channels[i]
			if(open){
				done = false
				pixels_processed += val
			}
		}
		progress_bar(int(pixels_processed))
	}

	fmt.Print("\rFinished Rendering in ", time.Since(t), " on ", cpus, " CPUs\n")

	//a paletted png can only use up to 255 colors
	//so it'll need converting
	if len(palette) > 255 {

	}

	imgFile, _ := os.Create("image.png")
	png.Encode(imgFile, img)
}
