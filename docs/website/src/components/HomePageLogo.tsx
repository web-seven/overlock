import React from "react";
function HomePageLogo(props) {

  return (
              <div className="hero-square-logo">
                <img
                  className="img-fluid"
                  src={props.logoSrc}
                  alt="hero-logo"
                />
              </div>
  );
}

export default HomePageLogo;
