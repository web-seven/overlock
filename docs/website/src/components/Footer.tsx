import React from "react";
import {useThemeConfig} from '@docusaurus/theme-common';

export const Footer = (props) => {
  const {footer} = useThemeConfig();
  const columns = props.columns || [];
  const emailItems = props.emailItems || [];
  const socialItems = props.socialItems || [];
  const contactsColumnTitle = props.contactsColumnTitle || "";
  const addressItems = props.addressItems || [];
 
  if (!footer) {
    return null;
  }
  const {copyright} = footer;

  return (
    <footer id="footer-3" className="pt-100">
      <div className="container">
        <div className="row">

          {columns.map((column, columnIndex) => (
            <div key={columnIndex} className={`col-sm-12 col-md-3 col-xl-3`}>
              <div className={`footer-links fl-${columnIndex + 1}`}>
                {column.map((link, index) => (
                  <React.Fragment key={index}>
                    {index === 0 && <h6 className="s-17 w-700">{link.title}</h6>}
                    <ul className="foo-links clearfix">
                      <li key={index}>
                        <p>
                          <a
                            style={{ color: "gray", textDecoration: "none" }}
                            href={link.url}
                          >
                            {link.linkTitle}
                          </a>
                        </p>
                      </li>
                    </ul>
                  </React.Fragment>
                ))}
              </div>
            </div>
          ))}

        </div>


        <div className="bottom-footer">
          <div className="row row-cols-12 row-cols-md-12 d-flex align-items-center">
            <div className="col">
              <div className="footer-copyright">
                <p className="p-sm text-center">{copyright}</p>
              </div>
            </div>

          </div>
        </div>
      </div>
    </footer>
  );
};

export default Footer;
