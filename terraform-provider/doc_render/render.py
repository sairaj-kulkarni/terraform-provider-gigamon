#! /usr/bin/env python3
from flask import Flask, request, render_template, make_response, redirect

# We maintain a session level state on whether the home page is visited or not. If we visit
# any internal page, without visting the home page, we then need to display the left side
# navigation along with the specific content of that page.

app = Flask(__name__)

def render_page(page_content):
    if request.cookies.get('visited') is None:
        resp = make_response(render_template('content.html', page_content=page_content))
        resp.set_cookie('visited', 'true')
        print ('setting the cookie')
    else:
        print ('cookie already set')
        resp = make_response(page_content)
    return resp

@app.route('/logout')
def logout():
    '''use this to clear out the cookies for this session'''
    resp = make_response(redirect('/'))
    resp.delete_cookie('visited')
    return resp

@app.route('/')
def home():
    page_content = r"<h1>Gigamon Terraform Provider</h1><p>Welcome to Gigamon Terraform     Provider</p>"
    return render_page(page_content)

@app.route('/about')
def about():
    page_content = r"<h1>About Content</h1><p>Welcome to your about page.</p>"
    return render_page(page_content)

@app.route('/contact')
def contact():
    page_content = r"<h1>Contact Content</h1><p>Welcome to your contact page.</p>"
    return render_page(page_content)

if __name__ == '__main__':
    app.run(debug=True, host="0.0.0.0", port=9999)

