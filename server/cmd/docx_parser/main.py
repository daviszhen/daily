#!/usr/bin/env python3
"""解析 docx 日报文档，按日期分段输出 JSON。
用法: python3 main.py <file.docx>
输出: [{"date":"2026年2月13日","text":"成员\t...\n曹凯\t...\n..."}, ...]
"""
import sys, json, re

def get_cell_text(cell):
    """提取单元格文本，避免重复"""
    from docx.oxml.ns import qn
    parts = []
    for p in cell.findall(qn('w:p')):
        runs = p.findall(qn('w:r'))
        text = ''.join(r.findtext(qn('w:t'), '') for r in runs)
        if text.strip():
            parts.append(text.strip())
    return '\n'.join(parts)

def get_para_text(elem):
    """提取段落文本，避免重复"""
    from docx.oxml.ns import qn
    runs = elem.findall(qn('w:r'))
    return ''.join(r.findtext(qn('w:t'), '') for r in runs).strip()

def parse_docx(path):
    from docx import Document
    from docx.oxml.ns import qn
    doc = Document(path)
    sections = []
    current_date = None
    current_rows = []

    for elem in doc.element.body:
        tag = elem.tag.split('}')[-1] if '}' in elem.tag else elem.tag

        if tag == 'p':
            text = get_para_text(elem)
            date_match = re.search(r'(\d{4})年(\d{1,2})月(\d{1,2})日', text)
            if date_match:
                if current_date and current_rows:
                    sections.append({"date": current_date, "text": '\n'.join(current_rows)})
                current_date = text
                current_rows = []
                continue

        if tag == 'tbl' and current_date:
            rows = elem.findall(qn('w:tr'))
            for row in rows:
                cells = row.findall(qn('w:tc'))
                cell_texts = [get_cell_text(c) for c in cells]
                current_rows.append('\t'.join(cell_texts))

    if current_date and current_rows:
        sections.append({"date": current_date, "text": '\n'.join(current_rows)})

    return sections

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Usage: python3 main.py <file.docx>", file=sys.stderr)
        sys.exit(1)
    json.dump(parse_docx(sys.argv[1]), sys.stdout, ensure_ascii=False)
