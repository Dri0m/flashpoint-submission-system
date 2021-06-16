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

function bindFancyUpload(url) {
    let uploadButton = document.querySelector("#upload-button")
    let fileInput = document.querySelector("#file-input")
    let progressBar = document.querySelector("#files-progress")
    let progressText = document.querySelector("#progress-text")

    uploadButton.addEventListener("click", function () {
        uploadButton.disabled = true
        progressBar.hidden = false
        progressText.hidden = false

        if (fileInput.files.length === 0) {
            alert("Error : No file selected")
            uploadButton.disabled = false
            return
        }

        // if(file.size > allowed_size_mb*1024*1024) {
        //     alert('Error : Exceeded size');
        //     return;
        // }

        let formData = new FormData()

        let files = fileInput.files
        for(let i=0; i<files.length; i++) {
            if (!(files[i].name.endsWith(".7z") || files[i].name.endsWith(".zip"))) {
                alert('Error : Incorrect file type')
                uploadButton.disabled = false
                return
            }
            formData.append("files", files[i])
        }

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
                progressText.innerHTML = "Processing and validating your submissions, please wait..."
            } else {
                progressText.innerHTML = `Progress: ${percent_complete.toFixed(3)}% Upload speed: ${uploadSpeed}kB/s`
            }

            t1 = t2
            p1 = p2
        });

        let handleEnd = function (e) {
            if (request.status !== 200) {
                let msg = `something went wrong: status ${request.status} - ${request.response}`
                progressText.innerHTML = msg
                alert(msg)
            } else {
                let msg = "Upload successful."
                progressText.innerHTML = msg
                alert(msg)
            }
            uploadButton.disabled = false
        }

        // AJAX request finished event
        request.addEventListener("load", handleEnd);
        request.addEventListener("error", handleEnd);
        request.addEventListener("timeout", handleEnd);

        // send POST request to server side script
        request.send(formData)
    });
}