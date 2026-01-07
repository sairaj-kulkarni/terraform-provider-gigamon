#! /usr/bin/env python3
from flask import Flask, render_template, session

# We maintain a session level state on whether the home page is visited or not. If we visit
# any internal page, without visting the home page, we then need to display the left side
# navigation along with the specific content of that page.

app = Flask(__name__)
# A secret key is required for session management
app.secret_key = 'strong secret key'

@app.route('/')
def home():
    session['visited_home'] = True
    return render_template('index.html', title='Home')

@app.route('/get_content/About')
def about():
    if session.get('visited_home') is None:
        return render_template('about.html', title='About')
    session['visited_home'] = True
    return "<h1>About Content</h1><p>Welcome to your about.</p>"

@app.route('/get_content/Contact')
def contact():
    if session.get('visited_home') is None:
        return render_template('contact.html', title='Content')
    session['visited_home'] = True
    return "<h1>Contact Content</h1><p>Welcome to your contact.</p>"

if __name__ == '__main__':
    app.run(debug=True, host="0.0.0.0", port=9999)

