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
    form.action = url
    form.method = "POST"
    form.submit()
}

function changePage(number) {
    let url = new URL(window.location.href)

    let currentPage = url.searchParams.get("page")
    let newPage = 1 + number
    if (currentPage !== null) {
        let parsed = parseInt(currentPage, 10)
        if (!isNaN(parsed)) {
            newPage = parsed + number
        }
    }

    url.searchParams.set("page", newPage.toString(10))
    window.location.href = url
}

function uploadHandler(url, files, i) {
    if (i >= files.length) {
        document.querySelector("#upload-button").disabled = false
        return
    }
    let progressBarsContainer = document.querySelector("#progress-bars-container")

    let file = files[i]

    let formData = new FormData()
    formData.append("files", file)

    let progressBar = document.createElement("progress");
    progressBar.max = 100
    progressBar.value = 0
    let progressText = document.createElement("span");
    progressBarsContainer.appendChild(progressText)
    progressBarsContainer.appendChild(progressBar)

    let request = new XMLHttpRequest();
    request.open("POST", url)

    let t1 = 1
    let t2 = 2
    let p1 = 0
    let p2 = 0

    // upload progress event
    request.upload.addEventListener("progress", function (e) {
        t2 = performance.now()
        p2 = e.loaded
        let percent_complete = (e.loaded / e.total) * 100
        progressBar.value = percent_complete

        let uploadSpeed = ((((p2 - p1) / (t2 - t1)) * 1000) / 1000).toFixed(1)

        if (e.loaded === e.total) {
            progressText.innerHTML = `${file.name}<br>Processing and validating file, please wait...`
        } else {
            progressText.innerHTML = `${file.name}<br>Progress: ${percent_complete.toFixed(3)}% Upload speed: ${uploadSpeed}kB/s`
        }

        t1 = t2
        p1 = p2
    });

    let handleEnd = function (e) {
        if (request.status !== 200) {
            progressText.innerHTML = `${file.name}<br>something went wrong: status ${request.status} - ${request.response}`
            progressText.style.color = "red"
        } else {
            progressText.innerHTML = `${file.name}<br>Upload successful.`
        }
        uploadHandler(url, files, i+1)
    }

    request.addEventListener("loadend", handleEnd);
    request.send(formData)
}

function bindFancierUpload(url) {
    let uploadButton = document.querySelector("#upload-button")
    let fileInput = document.querySelector("#file-input")

    uploadButton.addEventListener("click", function () {
        uploadButton.disabled = true

        if (fileInput.files.length === 0) {
            alert("Error : No file selected")
            uploadButton.disabled = false
            return
        }

        let files = fileInput.files
        for (let i = 0; i < files.length; i++) {
            if (!(files[i].name.endsWith(".7z") || files[i].name.endsWith(".zip"))) {
                alert('Error : Incorrect file type')
                uploadButton.disabled = false
                return
            }
        }

        uploadHandler(url, files, 0)
    });
}