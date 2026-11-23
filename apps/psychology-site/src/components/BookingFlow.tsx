import React, { useState, useEffect, useRef } from 'react';
import { PersianDatePicker, formatPersianDate, Modal } from '@psychology/ui';

import type { PublicTherapistItem, PublicAppointmentSlot, PublicAppointmentResponse } from '@psychology/api-client';
import { tehranDay } from '../lib/booking-date';

type SlotItem = PublicAppointmentSlot;

export interface BookingFlowProps {
  therapists: PublicTherapistItem[];
  initialSlots: SlotItem[];
  initialDate: string;
  today: string;
  initialSlotsUnavailable?: boolean;
  clinicAddress?: string;
  clinicPhone?: string;
  officeHours?: string;
}

function formatTehranDateStr(dateStr: string): string {
  try {
    return new Intl.DateTimeFormat('fa-IR', {
      timeZone: 'Asia/Tehran',
      weekday: 'long',
      month: 'long',
      day: 'numeric',
    }).format(new Date(dateStr));
  } catch {
    return dateStr;
  }
}

function formatTehranTimeStr(dateStr: string): string {
  try {
    return new Intl.DateTimeFormat('fa-IR', {
      timeZone: 'Asia/Tehran',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(dateStr));
  } catch {
    return dateStr;
  }
}

export const BookingFlow: React.FC<BookingFlowProps> = ({
  therapists,
  initialSlots,
  initialDate,
  today,
  initialSlotsUnavailable,
  clinicAddress,
  clinicPhone,
  officeHours,
}) => {
  const [selectedTherapistId, setSelectedTherapistId] = useState(therapists[0]?.id ?? '');
  const [selectedDate, setSelectedDate] = useState(initialDate);
  const [slots, setSlots] = useState<SlotItem[]>(initialSlots);
  const [loading, setLoading] = useState<boolean>(false);
  const [filterByDate, setFilterByDate] = useState<boolean>(true);

  // Modal State
  const [selectedSlot, setSelectedSlot] = useState<SlotItem | null>(null);
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false);
  const [guestName, setGuestName] = useState<string>('');
  const [guestPhone, setGuestPhone] = useState<string>('');
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [modalAlert, setModalAlert] = useState<string | null>(null);
  const [slotsUnavailable, setSlotsUnavailable] = useState(Boolean(initialSlotsUnavailable));
  const [rejected, setRejected] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const inFlight = useRef(false);
  const continueButton = useRef<HTMLButtonElement>(null);

  const selectedTherapist = therapists.find(th => th.id === selectedTherapistId);

  useEffect(() => {
    if (!selectedTherapistId || !selectedDate) return;
    const controller = new AbortController();
    setLoading(true);
    setSlotsUnavailable(false);
    const query = new URLSearchParams({ from_date: selectedDate });
    fetch(`/v1/public/therapists/${encodeURIComponent(selectedTherapistId)}/slots?${query}`, {
      signal: controller.signal, cache: 'no-store',
    }).then(async res => {
      const data = res.ok ? await res.json() : undefined;
      if (!Array.isArray(data)) throw new Error('Slot request unsuccessful');
      if (!controller.signal.aborted) setSlots(data);
    }).catch(err => {
      if (controller.signal.aborted) return;
      console.error('Public slots unavailable', err);
      setSlots([]);
      setSlotsUnavailable(true);
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [selectedTherapistId, selectedDate, refresh]);

  const currentSlots = slots.filter(slot => slot.therapist_id === selectedTherapistId &&
    Number.isFinite(Date.parse(slot.starts_at)) && Date.parse(slot.starts_at) > Date.now());
  const dateFilteredSlots = currentSlots.filter(slot => tehranDay(new Date(slot.starts_at)) === selectedDate);
  const displayedSlots = filterByDate ? dateFilteredSlots : currentSlots;

  const clearSelection = () => {
    setSlots([]);
    setSelectedSlot(null);
    setLoading(true);
  };
  const selectTherapist = (id: string) => {
    if (id === selectedTherapistId) return;
    clearSelection();
    setSelectedTherapistId(id);
  };

  const handleDateChange = (date: string) => {
    if (date === selectedDate) return;
    clearSelection();
    setSelectedDate(date);
    setFilterByDate(true);
    if (!date) setLoading(false);
  };

  const handleSlotClick = (slot: SlotItem) => {
    setSelectedSlot(slot);
    setModalAlert(null);
    setRejected(false);
  };

  const handleCloseModal = () => {
    if (inFlight.current) return;
    setIsModalOpen(false);
    if (rejected) setSelectedSlot(null);
    setGuestName('');
    setGuestPhone('');
    setModalAlert(null);
    requestAnimationFrame(() => {
      if (!continueButton.current?.disabled) continueButton.current?.focus();
      else document.querySelector<HTMLElement>('.therapist-card[aria-pressed="true"]')?.focus();
    });
  };

  const handleSubmitBooking = async (e: React.SyntheticEvent) => {
    e.preventDefault();
    if (!selectedSlot || inFlight.current || rejected ||
        !displayedSlots.some(slot => slot.id === selectedSlot.id && slot.status === 'available')) return;
    if (!guestName.trim()) {
      setModalAlert('نام و نام خانوادگی را وارد کنید.');
      return;
    }
    inFlight.current = true;

    setSubmitting(true);
    setModalAlert(null);

    try {
      const res = await fetch('/v1/public/appointments', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          therapist_id: selectedTherapistId,
          slot_id: selectedSlot.id,
          guest_name: guestName.trim(),
          guest_phone: guestPhone.trim(),
        }),
      });

      if (res.status === 201) {
        const data: PublicAppointmentResponse = await res.json();
        if (data.status !== 'pending' || !data.id || data.slot_id !== selectedSlot.id ||
            data.therapist_id !== selectedTherapistId) {
          setModalAlert('نتیجهٔ ثبت نوبت قابل تأیید نیست.');
          setRejected(true);
          return;
        }
        window.location.assign(`/appointments/${encodeURIComponent(data.id)}`);
        return;
      }

      const errData = await res.json().catch(() => ({}));
      if (res.status === 409 || errData.code === 'SLOT_IN_PAST' || errData.code === 'SLOT_NOT_FOUND') {
        setRejected(true);
        setSlots([]);
        setRefresh(value => value + 1);
        setModalAlert(
          'متأسفانه این سانس هم‌اکنون رزرو شده یا در دست هماهنگی با متقاضی دیگری است. لطفاً سانس آزاد دیگری را انتخاب فرمایید.'
        );
      } else if (res.status === 429) {
        setModalAlert(
          'تعداد درخواست‌های شما بیش از حد مجاز است. لطفاً چند دقیقه بعد مجدداً تلاش کنید.'
        );
      } else if (errData.code === 'INVALID_PHONE') {
        setModalAlert(
          'شماره موبایل واردشده نامعتبر است. لطفاً شماره صحیح ۱۱ رقمی (مثلاً ۰۹۱۲۳۴۵۶۷۸۹) وارد کنید.'
        );
      } else {
        setModalAlert(errData.message || 'خطا در ثبت نوبت. لطفاً دوباره تلاش فرمایید.');
      }
    } catch (err) {
      console.error('Failed to submit appointment:', err);
      setModalAlert('خطای ارتباط با سرور. لطفاً اتصال اینترنت خود را بررسی کنید.');
    } finally {
      inFlight.current = false;
      setSubmitting(false);
    }
  };

  return (
    <div className="grid w-full min-w-0 items-start gap-8 lg:grid-cols-12">
      <div className="min-w-0 lg:col-span-8">
      {/* 1. Therapist Selector */}
      <div className="mb-8">
        <div className="flex items-center justify-between mb-3">
          <label className="block font-bold text-base-content text-sm sm:text-base">
            ۱. انتخاب درمانگر:
          </label>
          <span className="text-xs text-base-content/60">
            {therapists.length} درمانگر فعال
          </span>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4" id="therapist-list">
          {therapists.map((th) => {
            const isSelected = th.id === selectedTherapistId;
            return (
              <div
                key={th.id}
                role="button"
                tabIndex={0}
                aria-pressed={isSelected}
                onClick={() => selectTherapist(th.id)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    selectTherapist(th.id);
                  }
                }}
                className={`therapist-card card bg-base-100 border-2 rounded-2xl p-5 cursor-pointer transition-all duration-200 shadow-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 ${
                  isSelected
                    ? 'border-primary bg-primary/5 shadow-sm ring-1 ring-primary'
                    : 'border-base-200 hover:border-base-300'
                }`}
                data-id={th.id}
                data-name={th.name}
                data-duration={th.session_duration_minutes}
              >
                <div className="flex items-start justify-between gap-2 mb-1.5">
                  <div className="font-bold text-base sm:text-lg text-base-content">
                    {th.name}
                  </div>
                  {isSelected && (
                    <span className="badge badge-primary badge-sm text-[11px] font-medium shrink-0">
                      انتخاب‌شده
                    </span>
                  )}
                </div>
                <div className="text-xs font-semibold text-primary mb-2">
                  {th.title}
                </div>
                <div className="text-[11px] text-base-content/80 bg-base-200/60 px-2.5 py-1 rounded-box inline-block">
                  مدت هر جلسه: <strong>{th.session_duration_minutes} دقیقه</strong>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* 2. Persian Calendar Date Filter (React Island) */}
      <div className="mb-8 card bg-base-100 border border-base-200 p-6 rounded-2xl shadow-xs">
        <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
          <div>
            <label className="block font-bold text-base-content text-sm sm:text-base mb-1">
              ۲. فیلتر تاریخ مشاوره (تقویم هجری شمسی):
            </label>
            <p className="text-xs text-base-content/60">
              برای جستجوی سانس‌های فعال، روز مورد نظر خود را در تقویم زیر تعیین نمایید:
            </p>
          </div>
          <div className="w-full sm:w-72">
            <PersianDatePicker
              value={selectedDate}
              minValue={today}
              onChange={handleDateChange}
              aria-label="تاریخ مشاوره"
              name="booking_preferred_date"
            />
          </div>
        </div>
      </div>

      {/* 3. Available Slots Section */}
      <div className="mb-10">
        <div className="flex justify-between items-center mb-3">
          <div className="flex items-center gap-2">
            <label className="font-bold text-base-content text-sm sm:text-base">
              ۳. انتخاب سانس آزاد (به وقت تهران):
            </label>
            {filterByDate && dateFilteredSlots.length > 0 && (
              <span className="badge badge-primary badge-sm text-xs font-medium">
                فیلتر روز: {formatPersianDate(selectedDate)}
              </span>
            )}
          </div>
          <div className="flex items-center gap-3">
            {currentSlots.length > 0 && (
              <button
                type="button"
                onClick={() => { setSelectedSlot(null); setFilterByDate(!filterByDate); }}
                className="btn btn-ghost btn-xs text-primary"
              >
                {filterByDate ? 'نمایش همه سانس‌ها' : 'فقط روز انتخابی'}
              </button>
            )}
          </div>
        </div>

        <div
          id="slots-container"
          className="card bg-base-100 border border-base-200 rounded-2xl p-6 min-h-[160px]"
        >
          {loading ? <p role="status">در حال دریافت سانس‌ها…</p> : slotsUnavailable ? (
            <div role="status">
              <p>دریافت سانس‌ها موقتاً ممکن نیست.</p>
              <button type="button" className="btn btn-outline btn-sm mt-3" onClick={() => setRefresh(value => value + 1)}>تلاش دوباره</button>
            </div>
          ) : displayedSlots.length === 0 ? (
            <div id="no-slots-msg" className="text-center text-base-content/60 py-8 text-sm">
              {filterByDate && currentSlots.length > 0 ? (
                <div>
                  <p className="mb-2">
                    در تاریخ انتخابی ({formatPersianDate(selectedDate)}) سانس آزادی ثبت نشده است.
                  </p>
                  <button
                    type="button"
                    onClick={() => setFilterByDate(false)}
                    className="btn btn-outline btn-primary btn-xs"
                  >
                    مشاهده سانس‌های موجود در روزهای دیگر ({currentSlots.length} سانس)
                  </button>
                </div>
              ) : (
                'در حال حاضر سانس بازی برای این درمانگر ثبت نشده است. لطفاً روزهای دیگر را بررسی فرمایید.'
              )}
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3" id="slots-grid">
              {displayedSlots.map((s) => {
                const isAvailable = s.status === 'available';
                if (!isAvailable) {
                  return (
                    <button
                      key={s.id}
                      type="button"
                      disabled
                      aria-disabled="true"
                      className="slot-btn slot-btn-unavailable border border-base-300 bg-base-200/50 text-base-content/40 rounded-xl p-3 text-center cursor-not-allowed transition-all duration-150 line-through opacity-60"
                      data-slot-id={s.id}
                      data-starts={s.starts_at}
                      data-ends={s.ends_at}
                      data-status={s.status}
                    >
                      <div className="text-xs font-semibold mb-1">
                        {formatTehranDateStr(s.starts_at)} (تکمیل‌شده)
                      </div>
                      <div className="text-sm font-bold font-mono" dir="ltr">
                        {formatTehranTimeStr(s.starts_at)} - {formatTehranTimeStr(s.ends_at)}
                      </div>
                    </button>
                  );
                }
                return (
                  <button
                    key={s.id}
                    type="button"
                    onClick={() => handleSlotClick(s)}
                    aria-pressed={selectedSlot?.id === s.id}
                    className="slot-btn border border-success/30 bg-success/10 hover:bg-success hover:text-success-content rounded-xl p-3 text-center cursor-pointer transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
                    data-slot-id={s.id}
                    data-starts={s.starts_at}
                    data-ends={s.ends_at}
                    data-status={s.status}
                  >
                    <div className="text-xs font-semibold text-success mb-1">
                      {formatTehranDateStr(s.starts_at)}
                    </div>
                    <div className="text-sm font-bold text-base-content font-mono" dir="ltr">
                      {formatTehranTimeStr(s.starts_at)} - {formatTehranTimeStr(s.ends_at)}
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </div>
      </div>

      </div>

      <aside className="min-w-0 space-y-5 lg:col-span-4 lg:sticky lg:top-28">
        <section id="booking-summary" aria-labelledby="booking-summary-title"
          className="card bg-base-100 rounded-2xl border border-base-200 p-5 shadow-xs sm:p-6">
          <h2 id="booking-summary-title" className="text-xl font-bold">خلاصه نوبت انتخابی</h2>
          <dl className="my-4 grid grid-cols-[auto_1fr] gap-x-4 gap-y-3 text-sm">
            <dt>شیوهٔ جلسه</dt><dd className="font-semibold">حضوری</dd>
            <dt>درمانگر</dt><dd className="font-semibold">{selectedTherapist?.name}</dd>
            <dt>تاریخ</dt><dd className="font-semibold">{selectedDate ? formatPersianDate(selectedSlot ? tehranDay(new Date(selectedSlot.starts_at)) : selectedDate) : 'انتخاب نشده'}</dd>
            <dt>سانس</dt><dd className="font-semibold">{selectedSlot && !rejected
              ? `${formatTehranTimeStr(selectedSlot.starts_at)} تا ${formatTehranTimeStr(selectedSlot.ends_at)}` : 'انتخاب نشده'}</dd>
          </dl>
          <button type="button" id="booking-continue" ref={continueButton} className="btn btn-primary rounded-full"
            disabled={!selectedSlot || rejected || loading} onClick={() => setIsModalOpen(true)}>
            ادامه و ثبت درخواست
          </button>
        </section>
        <section className="rounded-2xl bg-base-200 p-5 text-sm leading-relaxed">
          <h2 className="mb-2 font-bold">اصول رازداری و هماهنگی</h2>
          <p>پذیرش از شمارهٔ تماس برای هماهنگی و تأیید درخواست استفاده می‌کند.</p>
          {clinicAddress && <p className="mt-3">{clinicAddress}</p>}
          {clinicPhone && <p>{clinicPhone}</p>}
          {officeHours && <p>{officeHours}</p>}
        </section>
      </aside>

      {/* 4. Booking Modal using shared Modal from @psychology/ui */}
      <Modal
        isOpen={isModalOpen && selectedSlot !== null}
        onOpenChange={(open) => {
          if (!open) {
            handleCloseModal();
          }
        }}
        title="تکمیل درخواست نوبت مشاوره"
        size="md"
        isDismissable={!submitting}
      >
        {selectedSlot && (
          <div id="booking-modal-content">
            {modalAlert && (
              <div
                id="modal-alert"
                role="alert"
                className="alert alert-error text-xs mb-4 leading-relaxed"
              >
                {modalAlert}
              </div>
            )}

            <div className="bg-base-200/50 border border-base-200 rounded-box p-4 mb-5 text-xs">
              <div className="mb-1.5 flex justify-between">
                <span className="text-base-content/60">درمانگر انتخابی:</span>
                <strong id="modal-therapist-name" className="text-base-content">
                  {selectedTherapist?.name || '-'}
                </strong>
              </div>
              <div className="flex justify-between">
                <span className="text-base-content/60">زمان نوبت:</span>
                <strong id="modal-slot-time" className="text-primary font-mono" dir="ltr">
                  {`${formatTehranDateStr(selectedSlot.starts_at)} (${formatTehranTimeStr(
                    selectedSlot.starts_at
                  )} تا ${formatTehranTimeStr(selectedSlot.ends_at)})`}
                </strong>
              </div>
            </div>

            <form id="booking-form" onSubmit={handleSubmitBooking} className="flex flex-col gap-3.5">
              <input type="hidden" id="selected-therapist-id" value={selectedTherapistId} />
              <input type="hidden" id="selected-slot-id" value={selectedSlot.id} />

              <div>
                <label
                  htmlFor="guest_name"
                  className="label-text text-xs font-semibold text-base-content block mb-1"
                >
                  نام و نام خانوادگی مراجع <span className="text-error">*</span>
                </label>
                <input
                  type="text"
                  id="guest_name"
                  autoFocus
                  autoComplete="name"
                  required
                  value={guestName}
                  onChange={(e) => setGuestName(e.target.value)}
                  placeholder="مثال: سارا محمدی"
                  className="input input-bordered w-full text-base-content bg-base-100 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-1"
                />
              </div>

              <div>
                <label
                  htmlFor="guest_phone"
                  className="label-text text-xs font-semibold text-base-content block mb-1"
                >
                  شماره تلفن همراه (جهت هماهنگی نوبت) <span className="text-error">*</span>
                </label>
                <input
                  type="tel"
                  id="guest_phone"
                  autoComplete="tel-national"
                  required
                  dir="ltr"
                  value={guestPhone}
                  onChange={(e) => setGuestPhone(e.target.value)}
                  placeholder="۰۹۱۲۳۴۵۶۷۸۹"
                  className="input input-bordered w-full text-base-content bg-base-100 text-sm text-end focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-1"
                />
                <small className="block text-[11px] text-base-content/60 mt-1">
                  شماره همراه فقط جهت تماس پرسنل پذیرش استفاده شده و کاملاً محرمانه می‌ماند.
                </small>
              </div>

              <div className="flex gap-2 mt-2">
                <button
                  type="submit"
                  id="submit-booking-btn"
                  disabled={submitting || rejected || loading}
                  className="btn btn-primary btn-sm flex-1 font-bold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
                >
                  {submitting ? 'در حال ثبت...' : 'تأیید و ارسال درخواست نوبت'}
                </button>
                <button
                  type="button"
                  id="cancel-modal-btn"
                  disabled={submitting}
                  onClick={handleCloseModal}
                  className="btn btn-ghost btn-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
                >
                  انصراف
                </button>
              </div>
            </form>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default BookingFlow;
