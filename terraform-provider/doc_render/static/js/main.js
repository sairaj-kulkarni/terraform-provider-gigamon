function loadContent(endpoint) {
    const rightPane = document.getElementById('right-pane');
    
    // Fetch the new HTML content from a specific Flask route
    fetch(`${endpoint}`)
        .then(response => response.text())
        .then(html => {
            // Replace the content of the right-pane div
            rightPane.innerHTML = html;
        })
        .catch(error => {
            console.error('Error fetching content:', error);
            rightPane.innerHTML = '<p>Error loading content.</p>';
        });
}

