const editor = ace.edit("editor");
editor.setKeyboardHandler("ace/keyboard/vim");
editor.session.setMode("ace/mode/golang");

const runButton = document.getElementById("run");
const terminal = document.getElementById("terminal");

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
        terminal.innerText = json.stdout + json.stderr;
    });
}

setInterval(function() {
    // fetch("/tips", {
    //     method: "GET"
    // }).then(function (response) {
    //     return response.json();
    // }).then(function (json) {
    //     let tipsWindow = document.getElementById("tipsWindow")
    //     tipsWindow.innerText = json.message
    // })
    let tipsWindow = document.getElementById("tipsWindow")
    let rowContent = editor.session.getLine(editor.selection.getCursor().row);
    if (rowContent.includes("if")) {
        tipsWindow.innerText = "ifの使い方";
    } else if (rowContent.includes("switch")) {
        tipsWindow.innerText = "switchの使い方";
    } else {
        tipsWindow.innerText = "";
    }
}, 1000);
