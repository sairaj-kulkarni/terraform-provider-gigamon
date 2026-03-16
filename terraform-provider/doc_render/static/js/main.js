const divider = document.getElementById("drag-handle");
const sidebar = document.querySelector(".sidebar");

let isDragging = false;

divider.addEventListener("mousedown", () => {
  isDragging = true;
  document.body.style.cursor = "col-resize";
});

document.addEventListener("mousemove", (e) => {
  if (!isDragging) return;
  sidebar.style.width = e.clientX + "px";
});

document.addEventListener("mouseup", () => {
  isDragging = false;
  document.body.style.cursor = "default";
});

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

