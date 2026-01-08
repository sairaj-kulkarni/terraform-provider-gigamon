#! /usr/bin/env python3

'''
   A simplistic Flask based server to serve our documents to the users. This is temporary
   till we host this on a public terrafrom site. It will also be useful in future releases
   as the source of documenation for internal testing teams till it is officially released
'''

import os
import re
from collections import defaultdict
import argparse
from flask import Flask, request, render_template, make_response, redirect
import markdown

# pylint: disable = possibly-used-before-assignment

app = Flask(__name__)

'''
   We have two panes for the user. The left is a navigation pane to allow the user to
   select the resource/datasource that they want to see, and on clicking it, the right
   pane shows the doc for that object.
   When we display the data for the obect, we should only update the right pane and not
   the left pane. also if someone comes with a link directly to some inner page on their
   first visit, than both the panes should be shown.

   To handle this in a simplistic manner, we just use a cookies to find out if the user
   has already visited us or not. Of course this has issues when the user opens a new
   tab or opens another window etc, and in those cases it looks like the user has already
   visited us

   Without some frontend and more work, this is not easy to solve, so for now just providing
   a logout url that the user can click to reset their cookie and also their display. If the
   pane view for any reason is not proper, just use the logout url and things will be back
'''

# location of the docs dir in our repo
DOC_DIR = "fm_terraform_provider/terraform-provider/docs"

def get_supported_objects(base_dir):
    '''
    Takes the base directory of fm_terrafrom repo and returns the list of platforms
    supported as a list and also the objects that we currently support as a dict
    '''

    doc_path = os.path.join(base_dir, DOC_DIR)
    supported_resource_types = ["resources", "datasources", "actions"]

    platforms = set()
    obj_dict = {}
    # Scan thorugh all the directories in this path
    for res_type in supported_resource_types:
        res_path = os.path.join(doc_path, res_type)
        if os.path.isdir(res_path):
            res_list = sorted(os.listdir(res_path))
            for res in res_list:
                platform = res.split('_')[1]
                platforms.add(platform)
                if platform not in obj_dict:
                    obj_dict[platform] = defaultdict(list)
                obj_dict[platform][res_type].append(res)
    return (sorted(platforms), obj_dict)

def get_html_for_md(md_file):
    '''Convert the md file to html content'''

    with open(md_file, "r", encoding="utf-8") as f:
        markdown_content = f.read()

    html_content = markdown.markdown(markdown_content)

    with open("/tmp/index.html", "w", encoding="utf-8") as html_file:
        html_file.write(html_content)

    blockquote_start = re.compile(r'<blockquote>')
    blockquote_end = re.compile(r'</blockquote>')
    remove_para_start = re.compile(r'(<p>)(.*)')
    remove_para_end = re.compile(r'(.*)(</p>)')

    in_blockquote = False
    with open("/tmp/index.html", "r", encoding="utf-8") as html_file:
        with open("/tmp/modified_index.html", "w", encoding="utf-8") as u_file:
            line = html_file.readline()
            while line:
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
    with open("/tmp/modified_index.html", "r", encoding="utf-8") as h_file:
        html_content = h_file.read()
    return html_content
def render_page(md_file):
    '''
    Called to render a page to the customer. If the cookie is not set, we should render
    both the content of the page in the right pane, as well as the navigation data in the
    left pane. If the cookie is set, we just need to send the data only for the right pane

    In case we have to send the data for the left pane, than we need to get all the resources,
    datasources, and objects we support and list them, using the appropriate template.
    '''

    # Convert the given md file into html content
    html_content = get_html_for_md(os.path.join(args.base_dir, DOC_DIR, md_file))
    '''
    with open(os.path.join(args.base_dir, DOC_DIR, md_file), "r", encoding="utf-8") as f:
        markdown_content = f.read()
    html_content = markdown.markdown(markdown_content)
    '''

    if request.cookies.get('visited') is None:
        # Need to redner the navigation pane. Get the detaisl
        platform_list, objects = get_supported_objects(args.base_dir)
        resp = make_response(render_template(
            'content.html',
            page_content=html_content,
            platform_list=platform_list,
            objects=objects,
        ))
        resp.set_cookie('visited', 'true')
        print ('setting the cookie')
    else:
        print ('cookie already set')
        resp = make_response(html_content)
    return resp

@app.route('/logout')
def logout():
    '''use this to clear out the cookies for this session'''
    resp = make_response(redirect('/'))
    resp.delete_cookie('visited')
    return resp

# This is the base/root page of the docs and should display the info on the provider
@app.route('/')
def home():
    '''This handles the root dir or provider page handling'''
    md_file = "index.md"
    return render_page(md_file)

@app.route('/<platform>/<res_type>/<res_file>')
def res_content(platform, res_type, res_file):
    '''This is the path called when the user clicks on any of the left navigation items'''
    md_file = os.path.join(res_type, res_file)
    return render_page(md_file)

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument(
        '--base_dir',
        help='path to where the fm_terraform repository is present',
        required=True,
    )

    parser.add_argument(
        '--host',
        help='The host interface to listen on. Defaults to 0.0.0.0',
        default='0.0.0.0',
    )

    parser.add_argument(
        '--port',
        help='The port on which to listen. Defaults to 9999',
        type=int,
        default=9999,
    )

    args=parser.parse_args()

    # start the server
    app.run(debug=True, host=args.host, port=args.port)
