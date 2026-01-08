#! /usr/bin/env python3

import markdown
import re

with open("index.md", "r", encoding="utf-8") as md_file:
    markdown_content = md_file.read()

html_content = markdown.markdown(markdown_content)

with open("index.html", "w", encoding="utf-8") as html_file:
    html_file.write(html_content)

blockquote_start = re.compile(r'<blockquote>')
blockquote_end = re.compile(r'</blockquote>')
remove_para_start = re.compile(r'(<p>)(.*)')
remove_para_end = re.compile(r'(.*)(</p>)')

in_blockquote = False
with open("index.html", "r", encoding="utf-8") as html_file:
    with open("modified_index.html", "w", encoding="utf-8") as u_file:
        line = html_file.readline()
        while line:
            print (f'line at start: {line}')
            line = line.strip("\n")
            if in_blockquote:
                m = remove_para_start.search(line)
                if m:
                    line = m[2]+"\n"
                    u_file.write(line)
                    line = html_file.readline()
                    continue
                m = remove_para_end.search(line)
                if m:
                    line = m[1]+"\n"
                    u_file.write(line)
                    line = html_file.readline()
                m = blockquote_end.search(line)
                if m:
                    print ('blockquote stopped')
                    in_blockquote = False
                    u_file.write("</pre>\n")
                    line = html_file.readline()
                else:
                    line = line + "\n"
                    print (f'line in blockquote: {line}')
                    u_file.write(line)
                    line = html_file.readline()
            else:
                m = blockquote_start.search(line)
                if m:
                    in_blockquote = True
                    u_file.write("<pre>\n")
                else:
                    line = line + "\n"
                    u_file.write(line)
                line = html_file.readline()
