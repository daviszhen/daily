import React, { useState, useEffect, useRef } from 'react';

interface ModalProps {
  open: boolean;
  title: string;
  message?: string;
  inputMode?: boolean;
  inputPlaceholder?: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  onConfirm: (value?: string) => void;
  onCancel: () => void;
}

export function ConfirmModal({ open, title, message, inputMode, inputPlaceholder, confirmText = '确认', cancelText = '取消', danger, onConfirm, onCancel }: ModalProps) {
  const [value, setValue] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) { setValue(''); setTimeout(() => inputRef.current?.focus(), 50); }
  }, [open]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center" style={{ background: 'rgba(0,0,0,0.3)' }} onClick={onCancel}>
      <div className="bg-white rounded-xl shadow-lg p-6 w-96" onClick={e => e.stopPropagation()}>
        <h3 className="text-base font-semibold mb-2" style={{ color: '#2C2C2C' }}>{title}</h3>
        {message && <p className="text-sm mb-4" style={{ color: '#6B5E4F' }}>{message}</p>}
        {inputMode && (
          <input
            ref={inputRef}
            className="w-full rounded-lg px-3 py-2 text-sm mb-4 focus:outline-none"
            style={{ border: '1px solid #E5DDD0' }}
            placeholder={inputPlaceholder}
            value={value}
            onChange={e => setValue(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter' && value.trim()) onConfirm(value.trim()); }}
          />
        )}
        <div className="flex justify-end space-x-3">
          <button onClick={onCancel} className="px-4 py-1.5 text-sm rounded-lg transition-colors" style={{ color: '#6B5E4F', border: '1px solid #E5DDD0' }}>
            {cancelText}
          </button>
          <button
            onClick={() => onConfirm(inputMode ? value.trim() : undefined)}
            disabled={inputMode && !value.trim()}
            className="px-4 py-1.5 text-sm rounded-lg text-white transition-colors disabled:opacity-40"
            style={{ background: danger ? '#dc2626' : '#2C2C2C' }}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
