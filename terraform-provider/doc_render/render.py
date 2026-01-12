#! /usr/bin/env python3

'''
   A simplistic Flask based server to serve our documents to the users. This is temporary
   till we host this on a public terrafrom site. It will also be useful in future releases
   as the source of documenation for internal testing teams till it is officially released

   This also hosts the provider so that other users like the testing team and others can
   get the provider from here, till we host it on some public or well hosted private registry
'''

import os
import re
from collections import defaultdict
import argparse
import tempfile
import json
from flask import Flask, request, render_template, make_response, redirect, jsonify, send_file
import markdown

# pylint: disable=used-before-assignment

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
ARTIFACT_DIR = "fm_terraform_provider/terraform-provider/artifacts"

# This lists the resource types that we expose in the document. and also tracks any difference
# in directory naming and view of that name we give to the customer. for e.g. data-sources is
# the directory name, but we should that as datasources to the user
SUPPORTED_RESOURCE_TYPES = [
    ("resources", "resources"),
    ("data-sources", "datasources"),
    ("actions", "action"),
]

MAP_RES_TYPE_TO_DIR = {
    "datasources": "data-sources",
}

def get_supported_objects(base_dir):
    '''
    Takes the base directory of fm_terrafrom repo and returns the list of platforms
    supported as a list and also the objects that we currently support as a dict
    '''

    doc_path = os.path.join(base_dir, DOC_DIR)

    platforms = set()
    obj_dict = {}
    # Scan thorugh all the directories in this path
    for res_type in SUPPORTED_RESOURCE_TYPES:
        res_path = os.path.join(doc_path, res_type[0])
        if os.path.isdir(res_path):
            res_list = sorted(os.listdir(res_path))
            for res in res_list:
                platform = res.split('_')[1]
                platforms.add(platform)
                if platform not in obj_dict:
                    obj_dict[platform] = defaultdict(list)
                obj_dict[platform][res_type[1]].append(res)
    return (sorted(platforms), obj_dict)

def get_html_for_md(md_file):
    '''
    Convert the md file to html content.
    We use the markdown package to convert .md to .html syntax. We also do some
    modification of few html tags to be in sync with our local renderer
    '''

    with open(md_file, "r", encoding="utf-8") as f:
        markdown_content = f.read()

    html_content = markdown.markdown(markdown_content)

    code_start = re.compile(r'(<p><code>)(.*)')
    code_end = re.compile(r'(.*)(</code></p>)')


    with tempfile.NamedTemporaryFile(
        mode="w", encoding="utf-8", delete_on_close=False
    ) as html_tmp:
        html_tmp.write(html_content)
        html_tmp.close()
        with tempfile.NamedTemporaryFile(
            mode="w", encoding="utf-8", delete_on_close=False,
        ) as mhtml_tmp:
            with open(html_tmp.name, "r", encoding="utf-8") as rhdl:
                line = rhdl.readline()
                while line:
                    line = line.strip("\n")
                    m = code_start.search(line)
                    if m:
                        line = m[1]+"<pre>\n"
                        mhtml_tmp.write(line)
                    else:
                        m = code_end.search(line)
                        if m:
                            line = m[1]+"</pre></code></p>\n"
                            mhtml_tmp.write(line)
                        else:
                            mhtml_tmp.write(line+"\n")
                    line = rhdl.readline()

            mhtml_tmp.close()
            with open(mhtml_tmp.name, "r", encoding="utf-8") as mhdl:
                html_content = mhdl.read()
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

# THe following two endpoints provide the various documentation of the provider

# This is the base/root page of the docs and should display the info on the provider
@app.route('/')
def home():
    '''This handles the root dir or provider page handling'''
    md_file = "index.md"
    return render_page(md_file)

@app.route('/<platform>/<res_type>/<res_file>')
def res_content(platform, res_type, res_file):
    '''This is the path called when the user clicks on any of the left navigation items'''
    _ = platform
    md_file = os.path.join(MAP_RES_TYPE_TO_DIR.get(res_type,res_type), res_file)
    return render_page(md_file)


# The below endpoints implement the Terraform registry protocol
@app.route('/.well-known/terraform.json')
def tf_well_known():
    '''Return the location of where we are hosting the modules and providers'''
    resp = {
        "modules.v1": "https://tf-proj.gigamon.com/modules/v1",
        "providers.v1": "https://tf-proj.gigamon.com/providers/v1",
    }
    return jsonify(resp)

@app.route('/providers/gigamon/gigamon/versions')
def tf_get_versions():
    '''Get the currently available set of versions'''
    with open(
        os.path.join(args.base_dir, ARTIFACT_DIR, "version.json"),
        "r",
        encoding="utf-8",
    ) as fhdl:
        resp = json.loads(fhdl.read())
    return jsonify(resp)

@app.route('/providers/gigamon/gigamon/<version>/<action>/<os_type>/<arch_type>')
def get_download_details(version, action, os_type, arch_type):
    '''Get the details required to downlaod and verify this version'''

    _ = action
    # Convert the version last digits as a two digit number as per our conversion
    meta_file_name = f'terraform-provider-gigamon_{version}_{os_type}_{arch_type}' + ".meta"
    meta_file_path = os.path.join(args.base_dir, ARTIFACT_DIR, meta_file_name)
    with open(meta_file_path, "r", encoding="utf-8") as fhdl:
        resp = json.loads(fhdl.read())
    return jsonify(resp)

# Download the various file artifacts
@app.route('/terraform-provider-gigamon/2.0.0/<file_name>')
def download_files(file_name):
    '''download the provided file name'''
    file_path = os.path.join(args.base_dir, ARTIFACT_DIR, file_name)
    return send_file(file_path, as_attachment=True)

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
