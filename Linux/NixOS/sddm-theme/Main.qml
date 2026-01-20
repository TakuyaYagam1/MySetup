import QtQuick
import QtQuick.Window
import QtQuick.Controls
import QtMultimedia
import "components"

Item {
    id: root

    FontLoader {
        id: mainFont
        source: config.FontFile
    }

    height: Screen.height
    width: Screen.width

    Rectangle {
        anchors.fill: parent
        color: "black"
    }

    AudioOutput {
        id: audioOutput
        muted: true
        volume: 0
    }

    MediaPlayer {
        id: mediaPlayer
        source: config.VideoBackground
        loops: MediaPlayer.Infinite
        autoPlay: true
        playbackRate: 1.0
        audioOutput: audioOutput
        videoOutput: videoOutput
    }

    VideoOutput {
        id: videoOutput
        anchors.fill: parent
        fillMode: VideoOutput.PreserveAspectCrop 
        opacity: (mediaPlayer.playbackState === MediaPlayer.PlayingState) ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: 500 } }
    }

    Item {
        id: contentPanel
        anchors {
            fill: parent
            topMargin: config.Padding
            bottomMargin: config.Padding
            leftMargin: config.Padding
            rightMargin: config.Padding
        }

        DateTimePanel {
            id: dateTimePanel
            anchors { top: parent.top; left: parent.left }
        }
        
        LoginPanel {
            id: loginPanel
            anchors.fill: parent
        }
    }
}
