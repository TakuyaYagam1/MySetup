//@ pragma DefaultEnv QSG_RENDER_LOOP=threaded
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland

ShellRoot {
  id: root

  property var expectedScreens: []
  property real transitionProgress: 0.0

  readonly property string transitionState: controller.state
  readonly property bool inputBlocking: transitionState === "capturing"
    || transitionState === "captured"
    || transitionState === "revealing"
  readonly property bool frameVisible: transitionState === "captured"
    || transitionState === "revealing"

  function screenNames(screens) {
    const names = [];
    for (let i = 0; i < screens.length; i++) {
      names.push(screens[i].name);
    }
    return names;
  }

  function captureReady(screenName) {
    controller.captureReady(screenName);
  }

  function captureFailed(screenName) {
    controller.captureFailed(screenName);
  }

  function reveal() {
    if (!controller.reveal()) {
      return;
    }

    revealAnimation.restart();
  }

  function abort() {
    controller.abort();
  }

  TransitionController {
    id: controller
    onExitRequested: Qt.callLater(() => Qt.quit())
  }

  Component.onCompleted: {
    const screens = (Quickshell.screens || []).slice();
    controller.initialize(screenNames(screens));
    expectedScreens = screens;
  }

  Connections {
    target: Quickshell

    function onScreensChanged() {
      if (!controller.transitionModel) {
        return;
      }
      controller.checkScreens(root.screenNames(Quickshell.screens || []));
    }
  }

  IpcHandler {
    target: "shellTransition"

    function status(): string {
      return root.transitionState;
    }

    function reveal(): void {
      root.reveal();
    }

    function abort(): void {
      root.abort();
    }
  }

  NumberAnimation {
    id: revealAnimation
    target: root
    property: "transitionProgress"
    from: 0.0
    to: 1.0
    duration: controller.transitionModel ? controller.transitionModel.durationMs : 5000
    easing.type: Easing.InOutCubic

    onFinished: controller.completeReveal()
  }

  Variants {
    model: root.expectedScreens

    delegate: PanelWindow {
      id: window

      required property ShellScreen modelData
      property bool deliveredFrame: false

      screen: modelData
      visible: root.inputBlocking
      color: "transparent"

      anchors.top: true
      anchors.left: true
      anchors.right: true
      anchors.bottom: true

      WlrLayershell.layer: WlrLayer.Overlay
      WlrLayershell.keyboardFocus: root.inputBlocking
        ? WlrKeyboardFocus.Exclusive
        : WlrKeyboardFocus.None
      WlrLayershell.namespace: "wahrwelt-shell-transition-" + modelData.name
      WlrLayershell.exclusionMode: ExclusionMode.Ignore

      mask: Region {
        item: root.inputBlocking ? inputSurface : null
      }

      ScreencopyView {
        id: captureView
        anchors.fill: parent
        captureSource: window.modelData
        live: false
        paintCursor: false

        onHasContentChanged: {
          if (hasContent && !window.deliveredFrame) {
            window.deliveredFrame = true;
            root.captureReady(window.modelData.name);
          } else if (!hasContent && window.deliveredFrame) {
            root.captureFailed(window.modelData.name);
          }
        }
      }

      ShaderEffectSource {
        id: frozenTexture
        anchors.fill: parent
        sourceItem: captureView
        hideSource: true
        live: true
        visible: false
      }

      ShaderEffect {
        anchors.fill: parent
        visible: root.frameVisible

        property variant source: frozenTexture
        property real progress: root.transitionProgress
        property real cellSize: 0.04
        property real aspectRatio: width / Math.max(height, 1)
        property real centerX: 0.5
        property real centerY: 0.5

        fragmentShader: Qt.resolvedUrl("shaders/honeycomb.frag.qsb")
      }

      Item {
        id: inputSurface
        anchors.fill: parent
        visible: root.inputBlocking

        MouseArea {
          anchors.fill: parent
          acceptedButtons: Qt.AllButtons
        }
      }
    }
  }
}
