#! /usr/bin/env python3

import markdown

with open("example.md", "r") as md_file:
    markdown_content = md_file.read()

html_content = markdown.markdown(markdown_content)

with open("index.html", "w") as html_file:
    html_file.write(html_content)

