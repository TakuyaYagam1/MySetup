//@ pragma DefaultEnv QSG_RENDER_LOOP=threaded
pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Window
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import "transition-model.js" as TransitionModel

ShellRoot {
  id: root

  property var expectedScreens: []
  readonly property string targetProfile: Quickshell.env("WAHRWELT_SHELL_TRANSITION_TARGET_PROFILE") || ""
  property real outgoingProgress: 0.0
  property real bridgeProgress: 0.0
  property real incomingProgress: 0.0
  property bool coverFenceArmed: false
  property bool settleFenceArmed: false

  readonly property string transitionState: controller.state
  readonly property url targetLogoSource: controller.transitionModel
    ? Qt.resolvedUrl(controller.transitionModel.targetLogoAsset)
    : ""
  readonly property int consumedBridgeTicks: controller.transitionModel
    ? TransitionModel.bridgeTicksConsumed(controller.transitionModel, root.bridgeProgress)
    : 0
  readonly property bool surfaceVisible: transitionState === "capturing"
    || transitionState === "captured"
    || transitionState === "outgoing"
    || transitionState === "covered"
    || transitionState === "incoming"
    || transitionState === "settling"
  readonly property bool inputBlocking: transitionState === "capturing"
    || transitionState === "captured"
    || transitionState === "outgoing"
    || transitionState === "covered"
    || transitionState === "incoming"
  readonly property bool frameVisible: transitionState === "outgoing"
    || transitionState === "covered"
    || transitionState === "incoming"
    || transitionState === "settling"

  onTransitionStateChanged: {
    if (transitionState === "done") {
      doneExitTimer.restart();
    } else {
      doneExitTimer.stop();
    }
  }

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

  function beginTransition() {
    if (!controller.beginTransition()) {
      return;
    }

    outgoingProgress = 0.0;
    bridgeProgress = 0.0;
    incomingProgress = 0.0;
    outgoingAnimation.restart();
  }

  function enterCoveredBridge() {
    coverFenceArmed = true;
  }

  function beginCoveredBridge() {
    if (transitionState !== "covered") {
      abort();
      return;
    }

    bridgeProgress = 0.0;
    bridgeAnimation.restart();
  }

  function enterIncoming() {
    if (!controller.beginIncoming()) {
      abort();
      return;
    }

    incomingProgress = 0.0;
    incomingAnimation.restart();
  }

  function enterSettling() {
    if (!controller.beginSettling()) {
      abort();
      return;
    }

    settleFenceArmed = true;
  }

  function abort() {
    outgoingAnimation.stop();
    bridgeAnimation.stop();
    incomingAnimation.stop();
    coverFenceArmed = false;
    settleFenceArmed = false;
    controller.abort();
  }

  TransitionController {
    id: controller
    onCoveredReady: root.beginCoveredBridge()
    onExitRequested: Qt.callLater(() => Qt.quit())
  }

  Component.onCompleted: {
    const screens = (Quickshell.screens || []).slice();
    controller.initialize(screenNames(screens), targetProfile);
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

    function start(): void {
      root.beginTransition();
    }

    function abort(): void {
      root.abort();
    }
  }

  NumberAnimation {
    id: outgoingAnimation
    target: root
    property: "outgoingProgress"
    from: 0.0
    to: 1.0
    duration: controller.transitionModel
      ? controller.transitionModel.outgoingDurationMs
      : 3000
    easing.type: Easing.InOutCubic
    onFinished: root.enterCoveredBridge()
  }

  NumberAnimation {
    id: bridgeAnimation
    target: root
    property: "bridgeProgress"
    from: 0.0
    to: 1.0
    duration: controller.transitionModel
      ? controller.transitionModel.bridgeDurationMs
      : 0
    easing.type: Easing.Linear
    onFinished: root.enterIncoming()
  }

  NumberAnimation {
    id: incomingAnimation
    target: root
    property: "incomingProgress"
    from: 0.0
    to: 1.0
    duration: controller.transitionModel
      ? controller.transitionModel.incomingDurationMs
      : 3000
    easing.type: Easing.InOutCubic
    onFinished: root.enterSettling()
  }

  Timer {
    id: doneExitTimer
    interval: 2000
    repeat: false
    onTriggered: Qt.quit()
  }

  Variants {
    model: root.expectedScreens

    delegate: PanelWindow {
      id: window

      required property ShellScreen modelData
      property bool deliveredFrame: false

      screen: modelData
      visible: root.surfaceVisible
      color: "transparent"
      surfaceFormat.opaque: false

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

      Item {
        id: neutralVeilSource
        anchors.fill: parent

        Rectangle {
          anchors.fill: parent
          color: Qt.rgba(0.035, 0.043, 0.061, 1.0)
        }

        Image {
          id: targetLogo
          anchors.centerIn: parent
          width: Math.min(parent.width, parent.height) * 0.18
          height: width
          source: root.targetLogoSource
          fillMode: Image.PreserveAspectFit
          smooth: true
          mipmap: true
        }

        Row {
          id: bridgeTicks

          anchors.top: targetLogo.bottom
          anchors.topMargin: Math.max(18, Math.min(parent.width, parent.height) * 0.025)
          anchors.horizontalCenter: targetLogo.horizontalCenter
          spacing: Math.max(8, Math.min(parent.width, parent.height) * 0.01)
          visible: root.transitionState === "covered"

          Repeater {
            model: controller.transitionModel
              ? controller.transitionModel.bridgeTickCount
              : 0

            delegate: Rectangle {
              required property int index

              width: Math.max(18, Math.min(window.width, window.height) * 0.025)
              height: Math.max(4, Math.min(window.width, window.height) * 0.006)
              radius: height / 2
              color: "white"
              opacity: index < root.consumedBridgeTicks ? 0.18 : 0.95

              Behavior on opacity {
                NumberAnimation {
                  duration: 180
                  easing.type: Easing.OutCubic
                }
              }
            }
          }
        }
      }

      ShaderEffectSource {
        id: neutralVeilTexture
        anchors.fill: parent
        sourceItem: neutralVeilSource
        hideSource: true
        live: true
        recursive: true
        visible: false
      }

      ShaderEffect {
        anchors.fill: parent
        visible: root.frameVisible

        property variant source: neutralVeilTexture
        property real progress: root.transitionState === "incoming"
          || root.transitionState === "settling"
          ? root.incomingProgress
          : 0.0
        property real cellSize: 0.04
        property real aspectRatio: width / Math.max(height, 1)
        property real centerX: 0.5
        property real centerY: 0.5

        fragmentShader: Qt.resolvedUrl("shaders/honeycomb.frag.qsb")
      }

      ShaderEffect {
        anchors.fill: parent
        visible: root.transitionState === "outgoing"

        property variant source: frozenTexture
        property real progress: root.outgoingProgress
        property real cellSize: 0.04
        property real aspectRatio: width / Math.max(height, 1)
        property real centerX: 0.5
        property real centerY: 0.5

        fragmentShader: Qt.resolvedUrl("shaders/honeycomb.frag.qsb")
      }

      Item {
        id: frameProbe
        anchors.fill: parent
        visible: false
      }

      Connections {
        target: frameProbe.Window.window

        function onFrameSwapped() {
          if (root.coverFenceArmed && root.transitionState === "outgoing") {
            controller.coverFramePresented(window.modelData.name);
            if (root.transitionState === "covered") {
              root.coverFenceArmed = false;
            } else if (root.transitionState === "outgoing") {
              frameProbe.Window.window.update();
            }
          } else if (root.settleFenceArmed && root.transitionState === "settling") {
            controller.settlingFramePresented(window.modelData.name);
            if (root.transitionState === "settling") {
              frameProbe.Window.window.update();
            } else {
              root.settleFenceArmed = false;
            }
          }
        }
      }

      Connections {
        target: root

        function onCoverFenceArmedChanged() {
          if (root.coverFenceArmed && frameProbe.Window.window) {
            frameProbe.Window.window.update();
          }
        }

        function onSettleFenceArmedChanged() {
          if (root.settleFenceArmed && frameProbe.Window.window) {
            frameProbe.Window.window.update();
          }
        }
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
