// Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.

// Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, version 3 of the License.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/> 

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

