#! /usr/bin/env python3

import markdown
import re

with open("index.md", "r", encoding="utf-8") as md_file:
    markdown_content = md_file.read()

html_content = markdown.markdown(markdown_content)

with open("index.html", "w", encoding="utf-8") as html_file:
    html_file.write(html_content)

code_start = re.compile(r'(<p><code>)(.*)')
code_end = re.compile(r'(.*)(</code></p>)')

with open("index.html", "r", encoding="utf-8") as html_file:
    with open("modified_index.html", "w", encoding="utf-8") as u_file:
        line = html_file.readline()
        while line:
            line = line.strip("\n")
            m = code_start.search(line)
            if m:
                line = m[1]+"<pre>\n"
                u_file.write(line)
            else:
                m = code_end.search(line)
                if m:
                    line = m[1]+"</pre></code></p>\n"
                    u_file.write(line)
                else:
                    u_file.write(line+"\n")
            line = html_file.readline()

