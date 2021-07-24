function uploadHandler(url, files, i, step) {
    if (i >= files.length) {
        return
    }
    let progressBarsContainer = document.querySelector("#progress-bars-container")

    const file = files[i];

    let formData = new FormData()
    formData.append("files", file)

    let progressBar = document.createElement("progress");
    progressBar.max = 100
    progressBar.value = 0
    let progressText = document.createElement("span");
    progressText.style.fontWeight = "bold"
    progressText.style.fontSize = "90%"
    progressText.style.textShadow = "0 1px 1px rgba(0, 0, 0, 0.08)"
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
            progressText.innerHTML = `${file.name}<br>Upload failed!<br>Request status: ${request.status} - ${friendlyHttpStatus[request.status]}<br>Server response: ${request.response}`
            if (request.status === 409) {
                progressText.style.color = "orange"
            } else {
                progressText.style.color = "red"
            }
        } else {
            const obj = JSON.parse(request.response);
            progressText.innerHTML = `${file.name}<br>Upload successful. <a href="/web/submission/${obj["submission_id"]}">View Submission</a>`
        }
        uploadHandler(url, files, i + step, step)
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

        let uploadQueues = document.querySelector("#upload-queues")
        for (let i = 0; i < parseInt(uploadQueues.value); i++) {
            uploadHandler(url, files, i, parseInt(uploadQueues.value))
        }
    });
}