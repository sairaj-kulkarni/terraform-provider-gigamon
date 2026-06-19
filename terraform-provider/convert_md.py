#! /usr/bin/env python3

#  Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.
#
#  Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)
#
#  This program is free software: you can redistribute it and/or modify
#  it under the terms of the GNU General Public License as published by
#  the Free Software Foundation, version 3 of the License.
#
#  This program is distributed in the hope that it will be useful,
#  but WITHOUT ANY WARRANTY; without even the implied warranty of
#  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
#  GNU General Public License for more details.
#
#  You should have received a copy of the GNU General Public License
#  along with this program. If not, see <https://www.gnu.org/licenses/>

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

