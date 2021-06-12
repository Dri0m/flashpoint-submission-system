function sendDelete(url) {
    if (confirm(`Are you sure you want to (soft) delete '${url}'?`)) {
        sendRequest(url, "DELETE")
    }
}

function sendRequest(url, method) {
    let request = new XMLHttpRequest()
    request.open(method, url, false)

    try {
        request.onload = function () {
            if (request.status !== 204) {
                alert(`failed to ${method} '${url}' - status ${request.status}`)
            } else {
                alert(`${method} of '${url}' successful`)
                location.reload()
            }
        };
        request.send()
    } catch (err) {
        alert(`failed to ${method} '${url}' - exception '${err.message}'`)
    }
}

function controlAllCheckboxes(cb, className) {
    let checkboxes = document.getElementsByClassName(className);

    for (let i = 0; i < checkboxes.length; i++) {
        checkboxes[i].checked = cb.checked
    }
}

function batchDownloadFiles(checkboxClassName, attribute) {
    let checkboxes = document.getElementsByClassName(checkboxClassName);

    let url = "/submission-file-batch/"

    for (let i = 0; i < checkboxes.length; i++) {
        if (checkboxes[i].checked) {
            url += checkboxes[i].dataset[attribute] + ","
        }
    }

    url = url.slice(0, -1)
    window.location.href = url
}

function batchComment(checkboxClassName, attribute) {
    let checkboxes = document.getElementsByClassName(checkboxClassName);

    let url = "/submission-batch/"

    for (let i = 0; i < checkboxes.length; i++) {
        if (checkboxes[i].checked) {
            url += checkboxes[i].dataset[attribute] + ","
        }
    }

    url = url.slice(0, -1)
    url += "/comment"

    let form = document.getElementById("batch-comment")
    form.action=url
    form.method="POST"
    form.submit()
}