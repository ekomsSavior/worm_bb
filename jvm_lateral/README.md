# JVM Lateral Movement Module

## Overview

Adds JVM/Java-environment lateral movement capabilities to worm_bb, based on
advanced JDK serialization gadget techniques discovered by **b0t**:

- **this$0 injection** — abuse unchecked `capturedArgs` in `SerializedLambda` 
  to smuggle outer class references past SerialFilter
- **SubMap.readResolve() chain** — automatic RCE during deserialization with
  no `armed` flag, no custom `readObject`, fires inside `readObject()` return
- **TransformingComparator variant** — drop-in `commons-collections` chain for
  full RCE on classpath

## Techniques Ported

| Technique | Source | Target |
|-----------|--------|--------|
| SerializedLambda capturedArgs inject | b0t (JVM Internals Pt.1) | JDK 8-11 |
| SubMap.readResolve gadget | b0t (JVM Internals Pt.1) | JDK 8-11 |
| JMX deserialization delivery | worm_bb | Any JVM with JMX exposed |
| RMI-JRMP deserialization | worm_bb | RMI Registry / DGC |
| JNDI injection | worm_bb | Log4j / Spring / WebLogic |

## Usage

The module auto-scans subnets for Java services and delivers payloads.
Can be invoked standalone from C2:

```
C2> JVM_LATERAL --target 10.10.1.0/24 --technique submap
C2> JVM_LATERAL --target 10.10.1.50:1099 --technique rmi
C2> JVM_LATERAL --gadget templates --output ./payloads
```

## Gadget Chain Components

```
jvm_lateral/
├── jvm_lateral.go        # Main module (scanner + attacker + dispatcher)
├── java_serial.go        # Java serialization stream builder (Go-native)
├── gadgets.go            # Gadget chain definitions & payload assembly
├── chains.go             # Implementation of each gadget chain
├── payloads.go           # Payload templates (reverse shell, dropper, etc.)
└── gadgets/              # Pre-compiled Java gadget classes
    ├── SerializedLambda.class
    ├── TemplatesImpl.class  
    ├── TemplatesComp.class
    └── PayloadTranslet.java
```

## Limitations

- JDK version-dependent: lambda hashes change per build (mitigation: generate
  payloads offline per target JDK version, spray during ops)
- Java 17+ module system blocks some chains (TemplatesImpl access restricted)
- Requires either `commons-collections:TransformingComparator` on classpath
  or JDK internal class availability for full RCE

---

Based on research by **b0t** — "JVM Exploitation Pt.1: Serialized Lambdas, 
this$0 Injection, and Deserialization Gadgets"
