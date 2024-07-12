import React from "react";


function VideoTeaser(props) {
  const titleParts = props.titleParts || [];
  const subtitleParts = props.subtitleParts || [];

  return (

    <div className="hero-23-img video-preview ">
      <a className="video-popup1" href={props.videoSrc}></a>
      <img
        className="img-fluid"
        src={props.imageSrc}
        alt="video-preview"
      />
    </div>

  );
}

export default VideoTeaser;
