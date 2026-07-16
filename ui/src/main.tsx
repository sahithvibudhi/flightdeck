import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import ProtectedRoute from './components/ProtectedRoute';
import { ToastHost } from './components/Toast';
import Login from './pages/Login';
import Setup from './pages/Setup';
import Apps from './pages/Apps';
import Deploy from './pages/Deploy';
import AppDetail from './pages/AppDetail';
import Settings from './pages/Settings';
import { initTheme } from './lib/theme';
import './style.css';

initTheme();

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <ToastHost />
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/setup" element={<Setup />} />
        <Route path="/" element={<ProtectedRoute><Apps /></ProtectedRoute>} />
        <Route path="/deploy" element={<ProtectedRoute><Deploy /></ProtectedRoute>} />
        <Route path="/apps/:id" element={<ProtectedRoute><AppDetail /></ProtectedRoute>} />
        <Route path="/settings" element={<ProtectedRoute><Settings /></ProtectedRoute>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  </StrictMode>
);
