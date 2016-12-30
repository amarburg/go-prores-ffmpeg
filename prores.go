package prores

import "fmt"
import "github.com/amarburg/goav/avcodec"
import "github.com/amarburg/goav/avutil"
import "github.com/amarburg/goav/swscale"

import "image"

import "encoding/binary"
import "bytes"

import "C"
import "unsafe"

// DecodeProRes takes a byte slice containing a single ProRes frame, and uses goav (w/ ffmpeg) 
// to produce a Go RGBA image.   This function requires that the frame width and height be known
// in advance.   It returns a pointer to the new image if successful, or nil and an error if
// unsuccessful.
//
// Note this function is still pretty rough.
func DecodeProRes( buf []byte, width int, height int ) (* image.RGBA, error) {

  avcodec.AvcodecRegisterAll()
  prores := avcodec.AvcodecFindDecoder( avcodec.CodecId(avcodec.AV_CODEC_ID_PRORES) )
  if prores == nil { panic("Couldn't find ProRes codec")}

  if prores.AvCodecIsDecoder() != 1 { panic("Isn't a decoder")}


  ctx := prores.AvcodecAllocContext3()
  if ctx == nil { panic("Couldn't allocate context") }

  res := ctx.AvcodecOpen2(prores,nil)
  if res < 0 { panic(fmt.Sprintf("Couldn't open context (%d)",res))}

  packet := avcodec.AvPacketAlloc()
  packet.AvInitPacket()
  //if packet == nil { panic("Couldn't allocate packet") }

  res = packet.AvPacketFromData( (*uint8)(unsafe.Pointer(&buf[0])), len(buf) )
  if res < 0 { panic(fmt.Sprintf("Couldn't set avpacket data (%d)",res))}

  // And a frame to receive the data
  videoFrame := avutil.AvFrameAlloc()
  if videoFrame == nil { panic("Couldn't allocate destination frame") }

  ctx.Width = int32(width)
  ctx.Height = int32(height)

  got_picture := 0
  res  = ctx.AvcodecDecodeVideo2( (*avcodec.Frame)(unsafe.Pointer(videoFrame)), &got_picture, packet )

  //fmt.Printf("Got picture: %d\n", got_picture)
  //fmt.Printf("%#v\n",*videoFrame)

  //width,height := 1920,1080 //videoFrame.width, videoFrame.height

  if got_picture == 0 { panic(fmt.Sprintf("Didn't get a picture, err = %04x", -res)) }

  fmt.Printf("Image is %d x %d, format %d\n", width, height, int(ctx.Pix_fmt) )

  // Convert frame to RGB
  dest_fmt := int32(avcodec.AV_PIX_FMT_RGBA)
  flags := 0
  ctxtSws := swscale.SwsGetcontext(width, height, swscale.PixelFormat(ctx.Pix_fmt),
                                  width, height, swscale.PixelFormat(dest_fmt),
                                  flags, nil, nil, nil )
  if ctxtSws == nil { panic("Couldn't open swscale context")}
  defer swscale.SwsFreecontext( ctxtSws )

  videoFrameRgb := avutil.AvFrameAlloc()
  if videoFrameRgb == nil { panic("Couldn't allocate destination frame") }
  //defer avutil.AvFrameFree( videoFrameRgb )   // This causes a segfault

  videoFrameRgb.Width = ctx.Width
  videoFrameRgb.Height = ctx.Height
  videoFrameRgb.Format = dest_fmt

  avutil.AvFrameGetBuffer( videoFrameRgb, 8)

  swscale.SwsScale( ctxtSws, videoFrame.Data,
                             videoFrame.Linesize,
                             0, height,
                             videoFrameRgb.Data,
                             videoFrameRgb.Linesize )

  //fmt.Printf("%#v\n",*videoFrameRgb)

  // nb. C.GoBytes makes a copy of the data
  rgb_data :=  C.GoBytes(unsafe.Pointer(videoFrameRgb.Data[0]), C.int(videoFrameRgb.Width * videoFrameRgb.Height * 4))
  //pixels := make([]byte, videoFrameRgb.Width * videoFrameRgb.Height * 4 )

  reader := bytes.NewReader( rgb_data )
  img := image.NewRGBA( image.Rect(0,0,width,height) )
  err := binary.Read( reader, binary.LittleEndian, &img.Pix)

  if err != nil { panic(fmt.Sprintf("error on binary read: %s", err.Error() ))}

  //TODO:  Need to clean up all of my libav structures
avutil.AvFrameFree( videoFrameRgb )

  return img, nil
}
