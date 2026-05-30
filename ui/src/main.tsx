import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { Home } from "./pages/Home";
import { ListView } from "./pages/ListView";
import { Popout } from "./pages/Popout";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/lists/:slug" element={<ListView />} />
        <Route path="/popout/:slug" element={<Popout />} />
      </Routes>
    </BrowserRouter>
  </React.StrictMode>
);
