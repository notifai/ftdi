package ft2xxr

/*
#include <stdlib.h>
#include <libusb-1.0/libusb.h>

#cgo pkg-config: libusb-1.0
*/
import "C"

import (
	"bytes"
	"runtime"
	"unsafe"
)

var ctx *C.libusb_context

type USBError int

func (e USBError) Error() string {
	return C.GoString(C.libusb_error_name(C.int(e)))
}

func init() {
	if e := C.libusb_init(&ctx); e < 0 {
		panic(USBError(e))
	}
}

// Device represents some FT2xxR device
type Device struct {
	d    *C.libusb_device
	desc C.struct_libusb_device_descriptor
}

func (d *Device) unref() {
	C.libusb_unref_device(d.d)
}

func (d *Device) Connect() (*Conn, error) {
	var h *C.libusb_device_handle
	if e := C.libusb_open(d.d, &h); e < 0 {
		return nil, USBError(e)
	}
	return &Conn{h: h, d: d}, nil
}

func FindDevices(vendor, product int) ([]*Device, error) {
	var dl **C.libusb_device
	n := C.libusb_get_device_list(ctx, &dl)
	if n < 0 {
		return nil, USBError(n)
	}
	defer C.libusb_free_device_list(dl, 1)

	var found []*Device

	for _, d := range (*(*[1 << 30]*C.libusb_device)(unsafe.Pointer(dl)))[:n] {
		var desc C.struct_libusb_device_descriptor
		if e := C.libusb_get_device_descriptor(d, &desc); e < 0 {
			return nil, USBError(e)
		}
		if int(desc.idVendor) == vendor && int(desc.idProduct) == product {
			dev := &Device{d: C.libusb_ref_device(d), desc: desc}
			runtime.SetFinalizer(dev, (*Device).unref)
			found = append(found, dev)
		}
	}
	return found, nil
}

type Conn struct {
	h *C.libusb_device_handle
	d *Device
}

func (c *Conn) Description() (string, error) {
	buf := make([]byte, 256)
	e := C.libusb_get_string_descriptor_ascii(
		c.h, c.d.desc.iProduct, (*C.uchar)(&buf[0]), C.int(len(buf)),
	)
	if e < 0 {
		return "", USBError(e)
	}
	if i := bytes.IndexByte(buf, 0); i != -1 {
		buf = buf[:i]
	}
	return string(buf), nil
}

/*
// Open opens the index-th device with a given vendor id, product id,
// description and serial. Uses specified interface.
func Open(vendor, product int, description, serial string, index uint,
	c Channel) (*Device, error) {

	d, err := makeDevice(c)
	if err != nil {
		return nil, err
	}

	descr := C.CString(description)
	defer C.free(unsafe.Pointer(descr))
	ser := C.CString(serial)
	defer C.free(unsafe.Pointer(ser))

	e := C.ftdi_usb_open_desc_index(
		d.ctx,
		C.int(vendor), C.int(product),
		descr, ser,
		C.uint(index),
	)
	if e < 0 {
		defer d.deinit()
		return nil, d.makeError(e)
	}
	return d, nil
}


)func makeDevice(c Channel) (*Device, error) {
	d := new(Device)
	d.ctx = new(C.struct_ftdi_context)
	e := C.ftdi_init(d.ctx)
	if e < 0 {
		defer d.deinit()
		return nil, d.makeError(e)
	}
	if c != ChannelAny {
		e = C.ftdi_set_interface(d.ctx, C.enum_ftdi_interface(c))
		if e < 0 {
			defer d.deinit()
			return nil, d.makeError(e)
		}
	}
	runtime.SetFinalizer(d, (*Device).Close)
	return d, nil
}

func (d *Device) deinit() {
	C.ftdi_deinit(d.ctx)
}

func (d *Device) makeError(code C.int) error {
	if code >= 0 {
		return nil
	}
	return &Error{
		code: int(code),
		str:  C.GoString(C.ftdi_get_error_string(d.ctx)),
	}
}

// Close closes device
func (d *Device) Close() error {
	defer d.deinit()
	e := C.ftdi_usb_close(d.ctx)
	runtime.SetFinalizer(d, nil)
	return d.makeError(e)
}

type Channel uint32

const (
	ChannelAny Channel = iota
	ChannelA
	ChannelB
	ChannelC
	ChannelD
)

// OpenFirst opens the first device with a given vendor and product ids. Uses
// specified interface.
func OpenFirst(vendor, product int, c Channel) (*Device, error) {
	d, err := makeDevice(c)
	if err != nil {
		return nil, err
	}
	e := C.ftdi_usb_open(d.ctx, C.int(vendor), C.int(product))
	if e < 0 {
		defer d.deinit()
		return nil, d.makeError(e)
	}
	return d, nil
}

type Mode byte

const (
	ModeReset Mode = iota
	ModeBitbang
	ModeMPSSE
	ModeSyncBB
	ModeMCU
	ModeOpto
	ModeCBUS
	ModeSyncFF
	ModeFT1284
)

func (d *Device) SetBitmode(iomask byte, mode Mode) error {
	e := C.ftdi_set_bitmode(d.ctx, C.uchar(iomask), C.uchar(mode))
	return d.makeError(e)
}

func (d *Device) Reset() error {
	return d.makeError(C.ftdi_usb_reset(d.ctx))
}

func (d *Device) PurgeRxBuffer() error {
	return d.makeError(C.ftdi_usb_purge_rx_buffer(d.ctx))
}

func (d *Device) PurgeTxBuffer() error {
	return d.makeError(C.ftdi_usb_purge_tx_buffer(d.ctx))
}

func (d *Device) PurgeBuffers() error {
	return d.makeError(C.ftdi_usb_purge_buffers(d.ctx))
}
func (d *Device) WriteChunkSize() (int, error) {
	var cs C.uint
	e := C.ftdi_write_data_get_chunksize(d.ctx, &cs)
	return int(cs), d.makeError(e)
}

func (d *Device) SetWriteChunkSize(cs int) error {
	return d.makeError(C.ftdi_write_data_set_chunksize(d.ctx, C.uint(cs)))
}

func (d *Device) Write(buf []byte) (int, error) {
	n := C.ftdi_write_data(
		d.ctx,
		(*C.uchar)(unsafe.Pointer(&buf[0])),
		C.int(len(buf)),
	)
	if n < 0 {
		return 0, d.makeError(n)
	}
	return int(n), nil
}

func (d *Device) WriteByte(b byte) error {
	n := C.ftdi_write_data(d.ctx, (*C.uchar)(&b), 1)
	if n != 1 {
		return d.makeError(n)
	}
	return nil
}

func (d *Device) SetBaudrate(br int) error {
	return d.makeError(C.ftdi_set_baudrate(d.ctx, C.int(br)))
}

// EEPROM returns a handler to the device internal EEPROM subsystem
func (d *Device) EEPROM() EEPROM {
	return EEPROM{d}
}*/
