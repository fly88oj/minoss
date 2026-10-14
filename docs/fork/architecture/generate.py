#!/usr/bin/env python3
"""Generate the architecture renderings from extraction.model.json.

The JSON model is the source of truth (ids, labels, state, confidence,
sourceRefs). This script emits:
  - extraction-boundary.dot, iam-seam-blockers.dot   (Graphviz, reviewable text)
  - cmd-internal-extraction.drawio                  (editable Draw.io delivery)

Usage: python3 generate.py            # writes all three files next to this one
"""

import json
import os
import xml.sax.saxutils as xu

HERE = os.path.dirname(os.path.abspath(__file__))
MODEL = os.path.join(HERE, "extraction.model.json")

LINE_H = 15
PAD_X = 18
PAD_Y = 14
GAP_X = 60
GAP_Y = 110
MARGIN = 40
CHAR_W = 6.6


def node_size(node):
    lines = node["label"]
    w = max(len(str(l)) for l in lines) * CHAR_W + PAD_X * 2
    h = len(lines) * LINE_H + PAD_Y * 2
    return int(max(w, 150)), int(h)


def label(node):
    # draw.io stores HTML labels XML-escaped: the attribute must contain
    # &lt;br&gt; so that the renderer sees <br> as a line break.
    return "&lt;br&gt;".join(
        xu.escape(str(l), {'"': "&quot;"}) for l in node["label"])


def wrap_label(lines):
    return "\n".join('    "%s";' % l for l in lines)



def edge_meta(e):
    bits = ["type=%s" % e["type"], "confidence=%s" % e["confidence"]]
    for key in ("sync", "protocol", "risk"):
        if e.get(key):
            bits.append("%s=%s" % (key, e[key]))
    bits.append("sourceRefs: %s" % ("; ".join(e.get("sourceRefs", [])) or "-"))
    return " | ".join(bits)


def edge_label(e):
    # The risk marker lives in the model, not hand-prefixed into the label.
    if e.get("risk"):
        return "%s: %s" % (e["risk"], e["label"])
    return e["label"]


# ---------------------------------------------------------------- DOT
def emit_dot(view, path):
    out = [
        "// GENERATED from extraction.model.json by generate.py - do not edit.",
        "// View: %s" % view["title"],
        "// Question: %s" % view["question"],
        "// Level: %s" % view["level"],
        "digraph %s {" % view["id"].replace("-", "_"),
        "  rankdir=TB;",
        '  node [shape=box, fontname="Sans", fontsize=11];',
        '  edge [fontname="Sans", fontsize=10];',
    ]
    groups = {}
    for n in view["nodes"]:
        groups.setdefault(n.get("group", ""), []).append(n)

    idx = 0
    for group, nodes in groups.items():
        body = []
        for n in nodes:
            shape = "shape=cylinder" if n["type"] == "interface" else "shape=box"
            fill = ', style="filled", fillcolor="%s"' % n["fill"] if n.get("fill") else ""
            meta = "state=%s | confidence=%s | sourceRefs: %s" % (
                n["state"], n["confidence"], "; ".join(n["sourceRefs"]) or "-")
            body.append('  %s [%s%s, label=%s, tooltip="%s"];' % (
                n["id"], shape, fill,
                json.dumps("\\n".join(str(l) for l in n["label"])),
                xu.escape(meta, {'"': "&quot;"})))
            body.append('  // %s' % meta)
        if group:
            idx += 1
            out.append("  subgraph cluster_%d {" % idx)
            out.append("    label=%s;" % json.dumps(group))
            out.append("    style=rounded;")
            out.extend("  " + b for b in body)
            out.append("  }")
        else:
            out.extend(body)

    for e in view["edges"]:
        meta = edge_meta(e)
        attrs = ['label=%s' % json.dumps(edge_label(e))]
        if e.get("color"):
            attrs.append('color="%s"' % e["color"])
        if e.get("style") == "dashed":
            attrs.append("style=dashed")
        if e.get("style") == "bold":
            attrs.append("penwidth=2")
        attrs.append('tooltip="%s"' % xu.escape(meta, {'"': "&quot;"}))
        out.append("  %s -> %s [%s];" % (e["from"], e["to"], ", ".join(attrs)))
        out.append("  // %s" % meta)

    out.append("}")
    with open(path, "w") as f:
        f.write("\n".join(out) + "\n")


# ---------------------------------------------------------------- drawio
def layout(view):
    ranks = {}
    for n in view["nodes"]:
        ranks.setdefault(n["rank"], []).append(n)
    # column widths
    sizes = {n["id"]: node_size(n) for n in view["nodes"]}
    col_w = {r: sum(sizes[n["id"]][0] for n in ns) + GAP_X * (len(ns) - 1)
             for r, ns in ranks.items()}
    max_w = max(col_w.values())
    pos, y = {}, MARGIN
    for r in sorted(ranks):
        x = MARGIN + (max_w - col_w[r]) / 2
        row_h = max(sizes[n["id"]][1] for n in ranks[r])
        for n in ranks[r]:
            w, h = sizes[n["id"]]
            pos[n["id"]] = (int(x), int(y + (row_h - h) / 2), w, h)
            x += w + GAP_X
        y += row_h + GAP_Y
    return pos, max_w + 2 * MARGIN, y + MARGIN


def node_style(n):
    s = ["rounded=1", "whiteSpace=wrap", "html=1", "verticalAlign=middle",
         "align=center", "fontSize=11", "fontFamily=Helvetica"]
    if n.get("fill"):
        s.append("fillColor=%s" % n["fill"])
        s.append("strokeColor=#666666")
    if n["type"] == "interface":
        s.append("shape=cylinder3;boundedLbl=1;backgroundOutline=1;size=14")
    if n["type"] == "risk":
        s.append("dashed=1;strokeColor=#cc0000")
    if n.get("dashed"):
        s.append("dashed=1")
    if n["state"] in ("target", "proposed"):
        s.append("dashed=1;dashPattern=6 4")
    return ";".join(s) + ";"


def edge_style(e):
    s = ["edgeStyle=entityRelationEdgeStyle", "rounded=0", "orthogonalLoop=1",
         "jettySize=auto", "html=1", "fontSize=10", "fontFamily=Helvetica",
         "endArrow=block"]
    if e.get("color"):
        s.append("strokeColor=%s" % e["color"])
    else:
        s.append("strokeColor=#666666")
    if e.get("style") == "dashed":
        s.append("dashed=1")
    if e.get("style") == "bold":
        s.append("strokeWidth=2")
    return ";".join(s) + ";"


def emit_drawio(model, path):
    cells = []
    diagrams = []
    for view in model["views"]:
        pos, w, h = layout(view)
        body = []
        for n in view["nodes"]:
            x, y, nw, nh = pos[n["id"]]
            meta = "state=%s | confidence=%s | sourceRefs: %s" % (
                n["state"], n["confidence"], "; ".join(n["sourceRefs"]))
            body.append(
                '        <mxCell id="%s" value="%s" style="%s" vertex="1" parent="1">'
                '<mxGeometry x="%d" y="%d" width="%d" height="%d" as="geometry"/></mxCell>'
                % (n["id"], label(n), node_style(n), x, y, nw, nh))
            body.append('        <!-- %s -->' % meta)
        for e in view["edges"]:
            meta = edge_meta(e)
            body.append(
                '        <mxCell id="%s" value="%s" style="%s" edge="1" parent="1" '
                'source="%s" target="%s"><mxGeometry relative="1" as="geometry"/></mxCell>'
                % (e["id"], xu.escape(edge_label(e), {'"': "&quot;"}),
                   edge_style(e), e["from"], e["to"]))
            # draw.io has no verified slot for arbitrary metadata on a bare
            # mxCell, so the evidence travels with the file as a comment.
            body.append('        <!-- %s -->' % meta)
        diagrams.append(
            '    <diagram id="page-%s" name="%s">\n'
            '      <mxGraphModel dx="1200" dy="900" grid="1" gridSize="10" guides="1" '
            'tooltips="1" arrows="1" fold="1" page="1" pageScale="1" '
            'pageWidth="%d" pageHeight="%d" math="0" shadow="0">\n'
            '        <root>\n'
            '          <mxCell id="0"/>\n'
            '          <mxCell id="1" parent="0"/>\n'
            '%s\n'
            '        </root>\n'
            '      </mxGraphModel>\n'
            '    </diagram>' % (view["id"], xu.escape(view["title"]),
                                int(max(w, 1100)), int(max(h, 800)), "\n".join(body)))

    xml_out = (
        '<!-- GENERATED from extraction.model.json by generate.py - do not edit;\n'
        '     edit the model and regenerate. The .drawio file is a derived,\n'
        '     editable delivery artifact, not the canonical architecture model. -->\n'
        '<mxfile host="%s" version="24.0.0" type="device" pages="%d">\n%s\n</mxfile>\n'
        % ("app.diagrams.net", len(diagrams), "\n".join(diagrams)))
    with open(path, "w") as f:
        f.write(xml_out)
    cells.clear()


def main():
    with open(MODEL) as f:
        model = json.load(f)
    for view in model["views"]:
        emit_dot(view, os.path.join(HERE, "%s.dot" % view["id"]))
    emit_drawio(model, os.path.join(HERE, "cmd-internal-extraction.drawio"))
    print("generated:", ", ".join(
        ["%s.dot" % v["id"] for v in model["views"]] + ["cmd-internal-extraction.drawio"]))


if __name__ == "__main__":
    main()
