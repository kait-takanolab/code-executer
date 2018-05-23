const editor = ace.edit("editor");
editor.setKeyboardHandler("ace/keyboard/vim");
editor.session.setMode("ace/mode/golang");

const runButton = document.getElementById("run");
const console = document.getElementById("console");

runButton.addEventListener("click", RunCode, true);

function RunCode(e) {
    let code = editor.getValue();
    let runJson = {
        "code": code,
    };
    fetch("/compile", {
        method: "POST",
        body: JSON.stringify(runJson),
    }).then(function (response) {
        return response.json();
    }).then(function (json) {
        console.innerText = json.stdout;
    });
}