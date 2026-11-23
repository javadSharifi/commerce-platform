import React, { useState, useEffect, useRef } from 'react';
import { PersianDatePicker, formatPersianDate, Modal } from '@psychology/ui';

export interface TherapistItem {
  id: string;
  name: string;
  slug?: string;
  title: string;
  bio: string;
  session_duration_minutes: number;
}

export interface SlotItem {
  id: string;
  therapist_id: string;
  starts_at: string;
  ends_at: string;
  status: string;
}

export interface BookingFlowProps {
  therapists: TherapistItem[];
  initialSlots?: SlotItem[];
  initialTherapistId?: string;
  initialDate?: string;
  apiBase?: string;
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

function getTehranDateKey(dateStr: string): string {
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
      timeZone: 'Asia/Tehran',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).formatToParts(new Date(dateStr));
    const year = parts.find((p) => p.type === 'year')?.value;
    const month = parts.find((p) => p.type === 'month')?.value;
    const day = parts.find((p) => p.type === 'day')?.value;
    return `${year}-${month}-${day}`;
  } catch {
    return dateStr.slice(0, 10);
  }
}

function getTodayIsoTehran(): string {
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
      timeZone: 'Asia/Tehran',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).formatToParts(new Date());
    const year = parts.find((p) => p.type === 'year')?.value;
    const month = parts.find((p) => p.type === 'month')?.value;
    const day = parts.find((p) => p.type === 'day')?.value;
    return `${year}-${month}-${day}`;
  } catch {
    return new Date().toISOString().slice(0, 10);
  }
}

export const BookingFlow: React.FC<BookingFlowProps> = ({
  therapists = [],
  initialSlots = [],
  initialTherapistId,
  initialDate,
  apiBase = '',
}) => {
  const [selectedTherapistId, setSelectedTherapistId] = useState<string>(
    initialTherapistId || therapists[0]?.id || ''
  );
  const [selectedDate, setSelectedDate] = useState<string>(
    initialDate || getTodayIsoTehran()
  );
  const [slots, setSlots] = useState<SlotItem[]>(initialSlots);
  const [loading, setLoading] = useState<boolean>(false);
  const [filterByDate, setFilterByDate] = useState<boolean>(true);

  // Modal State
  const [selectedSlot, setSelectedSlot] = useState<SlotItem | null>(null);
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false);
  const [guestName, setGuestName] = useState<string>('');
  const [guestPhone, setGuestPhone] = useState<string>('');
  const [notes, setNotes] = useState<string>('');
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [modalAlert, setModalAlert] = useState<string | null>(null);

  // Track if component has hydrated
  const isHydrated = useRef(false);

  const selectedTherapist =
    therapists.find((th) => th.id === selectedTherapistId) || therapists[0];

  // Fetch live slots whenever therapist or date changes
  const fetchSlots = async (thId: string, dateStr: string) => {
    if (!thId) return;
    setLoading(true);
    try {
      const query = dateStr ? `?from_date=${encodeURIComponent(dateStr)}` : '';
      const host = typeof window !== 'undefined' ? window.location.host : '';
      const endpoint = `${apiBase}/v1/public/therapists/${thId}/slots${query}`;
      const res = await fetch(endpoint, {
        headers: host ? { Host: host } : {},
      });
      if (res.ok) {
        const data = await res.json();
        setSlots(Array.isArray(data) ? data : []);
      } else {
        setSlots([]);
      }
    } catch (err) {
      console.error('Error fetching therapist slots:', err);
      setSlots([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isHydrated.current) {
      isHydrated.current = true;
      // Fetch fresh slots on mount if on client to bypass any stale SSR data
      if (selectedTherapistId) {
        fetchSlots(selectedTherapistId, selectedDate);
      }
      return;
    }
    if (selectedTherapistId) {
      fetchSlots(selectedTherapistId, selectedDate);
    }
  }, [selectedTherapistId, selectedDate]);

  const handleTherapistSelect = (thId: string) => {
    setSelectedTherapistId(thId);
  };

  const handleDateChange = (isoDate: string) => {
    if (isoDate) {
      setSelectedDate(isoDate);
      setFilterByDate(true);
    }
  };

  // Filter slots for chosen date
  const dateFilteredSlots = slots.filter(
    (s) => getTehranDateKey(s.starts_at) === selectedDate
  );

  const displayedSlots =
    filterByDate && dateFilteredSlots.length > 0 ? dateFilteredSlots : slots;

  const handleSlotClick = (slot: SlotItem) => {
    setSelectedSlot(slot);
    setModalAlert(null);
    setIsModalOpen(true);
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
    setSelectedSlot(null);
    setModalAlert(null);
  };

  const handleSubmitBooking = async (e: React.SyntheticEvent) => {
    e.preventDefault();
    if (!selectedSlot || !selectedTherapistId) return;

    setSubmitting(true);
    setModalAlert(null);

    try {
      const host = typeof window !== 'undefined' ? window.location.host : '';
      const res = await fetch(`${apiBase}/v1/public/appointments`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(host ? { Host: host } : {}),
        },
        body: JSON.stringify({
          therapist_id: selectedTherapistId,
          slot_id: selectedSlot.id,
          guest_name: guestName.trim(),
          guest_phone: guestPhone.trim(),
          notes: notes.trim(),
        }),
      });

      if (res.status === 201) {
        const data = await res.json();
        try {
          sessionStorage.setItem(`appt_${data.id}_hash`, data.guest_lookup_hash);
        } catch (e) {
          console.error('sessionStorage error:', e);
        }
        window.location.href = `/appointments/${data.id}?hash=${encodeURIComponent(data.guest_lookup_hash)}`;
        return;
      }

      const errData = await res.json().catch(() => ({}));
      if (res.status === 409) {
        setModalAlert(
          'متأسفانه این سانس هم‌اکنون رزرو شده یا در دست هماهنگی با متقاضی دیگری است. لطفاً سانس آزاد دیگری را انتخاب فرمایید.'
        );
        fetchSlots(selectedTherapistId, selectedDate);
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
      setSubmitting(false);
    }
  };

  return (
    <div className="w-full max-w-full overflow-x-hidden">
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
                onClick={() => handleTherapistSelect(th.id)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    handleTherapistSelect(th.id);
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
                <p className="text-xs text-base-content/70 mb-3 leading-relaxed">
                  {th.bio}
                </p>
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
              defaultValue={initialDate || selectedDate}
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
            {slots.length > 0 && (
              <button
                type="button"
                onClick={() => setFilterByDate(!filterByDate)}
                className="btn btn-ghost btn-xs text-primary"
              >
                {filterByDate ? 'نمایش همه سانس‌ها' : 'فقط روز انتخابی'}
              </button>
            )}
            {loading && (
              <span className="text-xs text-primary inline-flex items-center gap-1">
                <span className="loading loading-spinner loading-xs"></span>
                در حال دریافت سانس‌ها...
              </span>
            )}
          </div>
        </div>

        <div
          id="slots-container"
          className="card bg-base-100 border border-base-200 rounded-2xl p-6 min-h-[160px]"
        >
          {displayedSlots.length === 0 ? (
            <div id="no-slots-msg" className="text-center text-base-content/60 py-8 text-sm">
              {filterByDate && slots.length > 0 ? (
                <div>
                  <p className="mb-2">
                    در تاریخ انتخابی ({formatPersianDate(selectedDate)}) سانس آزادی ثبت نشده است.
                  </p>
                  <button
                    type="button"
                    onClick={() => setFilterByDate(false)}
                    className="btn btn-outline btn-primary btn-xs"
                  >
                    مشاهده سانس‌های موجود در روزهای دیگر ({slots.length} سانس آزاد)
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
        isDismissable={true}
      >
        {selectedSlot && (
          <div id="booking-modal-content">
            {modalAlert && (
              <div
                id="modal-alert"
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

              <div>
                <label
                  htmlFor="notes"
                  className="label-text text-xs font-semibold text-base-content block mb-1"
                >
                  توضیحات یا علت مراجعه (اختیاری)
                </label>
                <textarea
                  id="notes"
                  rows={2}
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  placeholder="توضیح کوتاه درباره موضوع جلسه..."
                  className="textarea textarea-bordered w-full text-base-content bg-base-100 text-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-1"
                />
              </div>

              <div className="flex gap-2 mt-2">
                <button
                  type="submit"
                  id="submit-booking-btn"
                  disabled={submitting}
                  className="btn btn-primary btn-sm flex-1 font-bold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
                >
                  {submitting ? 'در حال ثبت...' : 'تأیید و ارسال درخواست نوبت'}
                </button>
                <button
                  type="button"
                  id="cancel-modal-btn"
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
