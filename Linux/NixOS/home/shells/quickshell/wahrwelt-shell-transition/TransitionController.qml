pragma ComponentBehavior: Bound

import QtQuick
import "transition-model.js" as TransitionModel

QtObject {
  id: controller

  property var transitionModel: null
  property string state: ""
  property int watchdogInterval: 0
  readonly property alias watchdogRunning: phaseWatchdog.running

  signal exitRequested()

  function armWatchdog() {
    phaseWatchdog.stop();
    watchdogInterval = TransitionModel.watchdogTimeoutMs(transitionModel);
    if (watchdogInterval > 0) {
      phaseWatchdog.start();
    }
  }

  function syncState() {
    const nextState = TransitionModel.status(transitionModel);
    if (nextState === state) {
      return;
    }

    state = nextState;
    armWatchdog();
  }

  function initialize(screenNames) {
    transitionModel = TransitionModel.create(screenNames);
    state = "";
    syncState();
    if (state === "aborted") {
      exitRequested();
    }
  }

  function captureReady(screenName) {
    if (TransitionModel.capture(transitionModel, screenName)) {
      syncState();
    }
  }

  function captureFailed(screenName) {
    if (!TransitionModel.captureFailed(transitionModel, screenName)) {
      return false;
    }

    syncState();
    exitRequested();
    return true;
  }

  function checkScreens(screenNames) {
    if (TransitionModel.screensMatch(transitionModel, screenNames)) {
      return true;
    }

    syncState();
    exitRequested();
    return false;
  }

  function reveal() {
    if (!TransitionModel.reveal(transitionModel)) {
      return false;
    }

    syncState();
    return true;
  }

  function abort() {
    if (!TransitionModel.abort(transitionModel)) {
      return false;
    }

    syncState();
    exitRequested();
    return true;
  }

  function completeReveal() {
    if (!TransitionModel.completeReveal(transitionModel)) {
      return false;
    }

    syncState();
    exitRequested();
    return true;
  }

  function expireWatchdog() {
    if (!TransitionModel.expireWatchdog(transitionModel)) {
      return false;
    }

    syncState();
    exitRequested();
    return true;
  }

  property Timer phaseWatchdog: Timer {
    id: phaseWatchdog
    interval: controller.watchdogInterval
    repeat: false
    onTriggered: controller.expireWatchdog()
  }
}
